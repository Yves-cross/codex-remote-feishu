import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { AdminRoute } from "./AdminRoute";
import {
  makeApp,
  makeBootstrap,
  makeFeishuManifest,
  makeImageStagingStatus,
  makeLogsStorageStatus,
  makeOnboardingWorkflow,
  makePreviewDriveStatus,
} from "../test/fixtures";
import { installMockFetch } from "../test/http";

type MockRoute = Parameters<typeof installMockFetch>[0][string];
type AdminWorkflowOverrides = Parameters<typeof makeOnboardingWorkflow>[0];

describe("AdminRoute", () => {
  it("keeps local workflow API requests dot-relative when mounted under a prefixed path", async () => {
    window.history.replaceState({}, "", "/g/demo/admin");
    const app = makeApp({ id: "bot-1", name: "Main Bot" });
    const prefix = "/g/demo/api/admin";

    const { calls } = installMockFetch(
      buildAdminRoutes({
        prefix,
        apps: [app],
        workflowRoutes: {
          [buildWorkflowPath(prefix, app.id)]: {
            body: makeAdminWorkflow(
              { id: app.id, name: app.name, appId: app.appId },
              {
                currentStage: "permission",
              },
            ),
          },
        },
      }),
    );

    render(<AdminRoute />);

    expect(
      await screen.findByRole("heading", {
        name: "Codex Remote Feishu v1.7.0 管理",
      }),
    ).toBeInTheDocument();
    expect(await screen.findByRole("heading", { name: "权限检查" })).toBeInTheDocument();
    await waitFor(() => {
      expect(calls.some((call) => call.path === buildWorkflowPath(prefix, app.id))).toBe(true);
    });
    expect(calls.every((call) => call.rawURL.startsWith("./"))).toBe(true);
    expectNoLegacyAdminReads(calls);
  });

  it("renders existing app detail from onboarding workflow and refreshes through the same workflow endpoint", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const app = makeApp({
      id: "bot-team",
      name: "协作机器人",
      appId: "cli_team",
    });
    let workflowReads = 0;
    const { calls } = installMockFetch(
      buildAdminRoutes({
        apps: [app],
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", app.id)]: () => {
            workflowReads += 1;
            if (workflowReads === 1) {
              return {
                body: makeAdminWorkflow(
                  {
                    id: app.id,
                    name: app.name,
                    appId: app.appId,
                  },
                  {
                    currentStage: "permission",
                    app: {
                      permission: {
                        status: "pending",
                        summary: "当前还缺少建议补齐的权限，请处理后重新检查。",
                        missingScopes: [{ scope: "drive:drive", scopeType: "tenant" }],
                      },
                    },
                  },
                ),
              };
            }
            return {
              body: makeAdminWorkflow(
                {
                  id: app.id,
                  name: app.name,
                  appId: app.appId,
                },
                {
                  currentStage: "events",
                  app: {
                    permission: {
                      status: "complete",
                      summary: "当前基础权限已经齐全。",
                      missingScopes: [],
                      grantJSON: "",
                    },
                    events: {
                      status: "pending",
                    },
                  },
                },
              ),
            };
          },
        },
      }),
    );

    render(<AdminRoute />);

    expect(await screen.findByRole("heading", { name: "权限检查" })).toBeInTheDocument();
    expect(
      await screen.findByText("这一步现在是建议补齐项，不会单独决定 workflow 下一步由谁 owner。"),
    ).toBeInTheDocument();
    expect(screen.getByText("drive:drive")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "重新检查" }));

    expect(await screen.findByRole("heading", { name: "事件订阅" })).toBeInTheDocument();
    expect(calls.some((call) => call.path.includes("/permission-check"))).toBe(false);
    expectNoLegacyAdminReads(calls);
  });

  it("creates a new robot and switches to workflow-driven detail after verify", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const existingApp = makeApp({ id: "bot-1", name: "主机器人", appId: "cli_main" });
    const newApp = makeApp({
      id: "bot-new",
      name: "运营机器人",
      appId: "cli_new",
      verifiedAt: "2026-04-25T09:10:00Z",
    });
    let appsConfigured = false;

    installMockFetch(
      buildAdminRoutes({
        appsRoute: (call) => {
          if (call.method === "POST") {
            appsConfigured = true;
            return {
              status: 201,
              body: {
                app: makeApp({
                  id: newApp.id,
                  name: newApp.name,
                  appId: newApp.appId,
                }),
              },
            };
          }
          return {
            body: {
              apps: appsConfigured ? [newApp] : [existingApp],
            },
          };
        },
        previews: {
          [existingApp.id]: makePreviewDriveStatus({
            gatewayId: existingApp.id,
            name: existingApp.name,
          }),
          [newApp.id]: makePreviewDriveStatus({
            gatewayId: newApp.id,
            name: newApp.name,
          }),
        },
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", existingApp.id)]: {
            body: makeAdminWorkflow(
              {
                id: existingApp.id,
                name: existingApp.name,
                appId: existingApp.appId,
              },
              {
                currentStage: "permission",
              },
            ),
          },
          [buildWorkflowPath("/api/admin", newApp.id)]: {
            body: makeAdminWorkflow(
              {
                id: newApp.id,
                name: newApp.name,
                appId: newApp.appId,
                verifiedAt: newApp.verifiedAt,
              },
              {
                currentStage: "permission",
              },
            ),
          },
        },
        extraRoutes: {
          "/api/admin/feishu/apps/bot-new/verify": {
            body: {
              app: newApp,
              result: { connected: true, duration: 1_000_000_000 },
            },
          },
        },
      }),
    );

    render(<AdminRoute />);

    await user.click(await screen.findByRole("button", { name: /新增机器人/ }));
    expect(await screen.findByRole("button", { name: "扫码创建" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "手动输入" }));
    await user.type(screen.getByLabelText("机器人名称（可选）"), "运营机器人");
    await user.type(screen.getByLabelText("App ID"), "cli_new");
    await user.type(screen.getByLabelText("App Secret"), "secret_new");
    await user.click(screen.getByRole("button", { name: "验证并保存" }));

    expect(await screen.findByRole("heading", { name: "运营机器人" })).toBeInTheDocument();
    expect(await screen.findByText(/连接测试成功，用时 1\.0s。/)).toBeInTheDocument();
  });

  it("opens the delete modal and removes the selected robot", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const app = makeApp({
      id: "bot-delete",
      name: "待删除机器人",
      appId: "cli_delete",
    });
    let removed = false;

    installMockFetch(
      buildAdminRoutes({
        appsRoute: () => ({
          body: {
            apps: removed ? [] : [app],
          },
        }),
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", app.id)]: {
            body: makeAdminWorkflow(
              {
                id: app.id,
                name: app.name,
                appId: app.appId,
              },
              {
                currentStage: "permission",
              },
            ),
          },
        },
        extraRoutes: {
          "/api/admin/feishu/apps/bot-delete": () => {
            removed = true;
            return { body: {} };
          },
        },
      }),
    );

    render(<AdminRoute />);

    await user.click(await screen.findByRole("button", { name: "删除机器人" }));
    expect(await screen.findByRole("dialog")).toHaveTextContent("确认删除机器人");
    await user.click(screen.getByRole("button", { name: "确认删除" }));

    expect(await screen.findByRole("heading", { name: "新增机器人" })).toBeInTheDocument();
    expect(await screen.findByText("机器人已删除。")).toBeInTheDocument();
  });

  it("surfaces event-test delivery errors from the workflow-owned events stage", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const app = makeApp({ id: "bot-1", name: "主机器人", appId: "cli_main" });

    installMockFetch(
      buildAdminRoutes({
        apps: [app],
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", app.id)]: {
            body: makeAdminWorkflow(
              {
                id: app.id,
                name: app.name,
                appId: app.appId,
              },
              {
                currentStage: "events",
                app: {
                  permission: {
                    status: "complete",
                    summary: "当前基础权限已经齐全。",
                    missingScopes: [],
                    grantJSON: "",
                  },
                },
              },
            ),
          },
        },
        extraRoutes: {
          "/api/admin/feishu/apps/bot-1/test-events": {
            status: 409,
            body: {
              error: {
                code: "feishu_app_web_test_recipient_unavailable",
                message: "recipient unavailable",
                details:
                  "手动添加的机器人无法自动发送测试消息，请直接在飞书后台继续手动配置。",
              },
            },
          },
        },
      }),
    );

    render(<AdminRoute />);

    await user.click(await screen.findByRole("button", { name: "发送测试提示" }));

    expect(
      await screen.findByText(
        "手动添加的机器人无法自动发送测试消息，请直接在飞书后台继续手动配置。",
      ),
    ).toBeInTheDocument();
  });

  it("records deferred autostart decisions through the admin onboarding workflow endpoint", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const app = makeApp({ id: "bot-1", name: "主机器人", appId: "cli_main" });
    let workflowReads = 0;

    installMockFetch(
      buildAdminRoutes({
        apps: [app],
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", app.id)]: () => {
            workflowReads += 1;
            if (workflowReads === 1) {
              return {
                body: makeAdminWorkflow(
                  {
                    id: app.id,
                    name: app.name,
                    appId: app.appId,
                  },
                  {
                    currentStage: "autostart",
                  },
                ),
              };
            }
            return {
              body: makeAdminWorkflow(
                {
                  id: app.id,
                  name: app.name,
                  appId: app.appId,
                },
                {
                  currentStage: "vscode",
                  autostart: {
                    status: "deferred",
                    summary: "你选择稍后再处理自动启动。",
                    allowedActions: ["apply", "record_enabled"],
                    decision: {
                      value: "deferred",
                      decidedAt: "2026-04-25T08:20:00Z",
                    },
                  },
                  guide: {
                    remainingManualActions: ["决定如何处理这台机器上的 VS Code 集成。"],
                  },
                },
              ),
            };
          },
        },
        extraRoutes: {
          "/api/admin/onboarding/machine-decisions/autostart": {
            status: 200,
            body: {},
          },
        },
      }),
    );

    render(<AdminRoute />);

    expect(await screen.findByRole("heading", { name: "自动启动" })).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "稍后处理" }));

    expect(await screen.findByRole("heading", { name: "VS Code 集成" })).toBeInTheDocument();
    expect(await screen.findByText("自动启动已留待稍后处理。")).toBeInTheDocument();
  });

  it("cleans up logs and updates the visible count", async () => {
    window.history.replaceState({}, "", "/admin");
    const user = userEvent.setup();
    const app = makeApp({ id: "bot-1", name: "主机器人", appId: "cli_main" });

    installMockFetch(
      buildAdminRoutes({
        apps: [app],
        logs: makeLogsStorageStatus({
          fileCount: 128,
          totalBytes: 860 * 1024 * 1024,
        }),
        workflowRoutes: {
          [buildWorkflowPath("/api/admin", app.id)]: {
            body: makeAdminWorkflow(
              {
                id: app.id,
                name: app.name,
                appId: app.appId,
              },
              {
                currentStage: "permission",
              },
            ),
          },
        },
        extraRoutes: {
          "/api/admin/storage/logs/cleanup": {
            body: {
              rootDir: "/tmp/logs",
              olderThanHours: 24,
              deletedFiles: 70,
              deletedBytes: 440 * 1024 * 1024,
              remainingFileCount: 58,
              remainingBytes: 420 * 1024 * 1024,
            },
          },
        },
      }),
    );

    render(<AdminRoute />);

    expect(await screen.findByText("128 个文件，约 860 MB")).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: "清理一天前日志" }));
    expect(await screen.findByText("58 个文件，约 420 MB")).toBeInTheDocument();
  });
});

function buildWorkflowPath(prefix: string, appID: string): string {
  return `${prefix}/onboarding/workflow?app=${encodeURIComponent(appID)}`;
}

function buildAdminRoutes(options?: {
  prefix?: string;
  apps?: ReturnType<typeof makeApp>[];
  appsRoute?: MockRoute;
  workflowRoutes?: Record<string, MockRoute>;
  previews?: Record<string, ReturnType<typeof makePreviewDriveStatus>>;
  logs?: ReturnType<typeof makeLogsStorageStatus>;
  image?: ReturnType<typeof makeImageStagingStatus>;
  extraRoutes?: Record<string, MockRoute>;
}) {
  const prefix = options?.prefix || "/api/admin";
  const apps = options?.apps || [];
  const routes: Record<string, MockRoute> = {
    [`${prefix}/bootstrap-state`]: {
      body: makeBootstrap(),
    },
    [`${prefix}/feishu/manifest`]: {
      body: makeFeishuManifest(),
    },
    [`${prefix}/feishu/apps`]: options?.appsRoute || {
      body: { apps },
    },
    [`${prefix}/storage/image-staging`]: {
      body: options?.image || makeImageStagingStatus(),
    },
    [`${prefix}/storage/logs`]: {
      body: options?.logs || makeLogsStorageStatus(),
    },
  };

  for (const app of apps) {
    routes[`${prefix}/storage/preview-drive/${encodeURIComponent(app.id)}`] =
      options?.previews?.[app.id]
        ? {
            body: options.previews[app.id],
          }
        : {
        body: makePreviewDriveStatus({
          gatewayId: app.id,
          name: app.name,
        }),
      };
  }

  return {
    ...routes,
    ...(options?.workflowRoutes || {}),
    ...(options?.extraRoutes || {}),
  };
}

function makeAdminWorkflow(
  appOverrides: Partial<ReturnType<typeof makeApp>> = {},
  overrides: AdminWorkflowOverrides = {},
) {
  if (overrides.app === null) {
    return makeOnboardingWorkflow({
      ...overrides,
      apps: overrides.apps || [],
      selectedAppId: overrides.selectedAppId || "",
      app: null,
    });
  }
  const app = makeApp({
    ...appOverrides,
    ...(overrides.app?.app || {}),
  });
  return makeOnboardingWorkflow({
    ...overrides,
    apps: overrides.apps || [app],
    selectedAppId: overrides.selectedAppId || app.id,
    app: {
      ...overrides.app,
      app,
    },
  });
}

function expectNoLegacyAdminReads(
  calls: Array<{
    path: string;
  }>,
) {
  expect(calls.some((call) => call.path.includes("/permission-check"))).toBe(false);
  expect(calls.some((call) => call.path.endsWith("/autostart/detect"))).toBe(false);
  expect(calls.some((call) => call.path.endsWith("/vscode/detect"))).toBe(false);
}
