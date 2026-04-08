import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { SetupRoute } from "./SetupRoute";
import { makeApp, makeBootstrap, makeManifest, makeVSCodeDetect } from "../test/fixtures";
import { installMockFetch } from "../test/http";

describe("SetupRoute", () => {
  it("shows read-only connect state and disables credential inputs", async () => {
    window.history.replaceState({}, "", "/setup");

    installMockFetch({
      "/api/setup/bootstrap-state": { body: makeBootstrap() },
      "/api/setup/feishu/apps": {
        body: {
          apps: [
            makeApp({
              readOnly: true,
              readOnlyReason: "当前由运行时环境变量接管，只能做连接测试。",
              wizard: {},
            }),
          ],
        },
      },
      "/api/setup/feishu/manifest": { body: { manifest: makeManifest() } },
      "/api/setup/vscode/detect": { body: makeVSCodeDetect() },
    });

    render(<SetupRoute />);

    expect(screen.getByText("正在读取最新状态")).toBeInTheDocument();
    expect(await screen.findByText("当前应用由运行时环境变量接管，setup 页面会直接对它做连接测试，但不会修改本地配置。")).toBeInTheDocument();
    expect(screen.getByLabelText("显示名称")).toBeDisabled();
    expect(screen.getByLabelText("App ID")).toBeDisabled();
    expect(screen.getByLabelText("App Secret")).toBeDisabled();
    expect(screen.getByRole("button", { name: "测试并继续" })).toBeEnabled();
  });

  it("lands on permissions after verify and blocks continue until confirmed", async () => {
    window.history.replaceState({}, "", "/setup");
    const { calls } = installMockFetch({
      "/api/setup/bootstrap-state": { body: makeBootstrap() },
      "/api/setup/feishu/apps": {
        body: {
          apps: [
            makeApp({
              wizard: {
                connectionVerifiedAt: "2026-04-08T00:00:00Z",
              },
            }),
          ],
        },
      },
      "/api/setup/feishu/manifest": { body: { manifest: makeManifest() } },
      "/api/setup/vscode/detect": { body: makeVSCodeDetect() },
    });

    render(<SetupRoute />);

    expect(await screen.findByText("权限导入说明")).toBeInTheDocument();

    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: "继续" }));

    const dialog = await screen.findByRole("dialog");
    expect(dialog).toHaveTextContent("请先在飞书平台完成权限导入，并勾选页面上的确认项。");
    expect(calls.some((call) => call.method === "PATCH" && call.path.includes("/wizard"))).toBe(false);
  });
});
