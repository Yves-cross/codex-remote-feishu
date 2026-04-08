import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { AdminRoute } from "./AdminRoute";
import {
  makeApp,
  makeBootstrap,
  makeImageStagingStatus,
  makeManifest,
  makePreviewDriveStatus,
  makeRuntimeStatus,
  makeVSCodeDetect,
} from "../test/fixtures";
import { installMockFetch } from "../test/http";

describe("AdminRoute", () => {
  it("shows the admin error state when bootstrap loading fails", async () => {
    installMockFetch({
      "/api/admin/bootstrap-state": {
        status: 500,
        body: { error: { message: "bootstrap load failed" } },
      },
      "/api/admin/runtime-status": { body: makeRuntimeStatus() },
      "/api/admin/feishu/apps": { body: { apps: [] } },
      "/api/admin/feishu/manifest": { body: { manifest: makeManifest() } },
      "/api/admin/vscode/detect": { body: makeVSCodeDetect() },
      "/api/admin/instances": { body: { instances: [] } },
      "/api/admin/storage/image-staging": { body: makeImageStagingStatus() },
    });

    render(<AdminRoute />);

    expect(screen.getByText("正在读取最新状态")).toBeInTheDocument();
    expect(await screen.findByText("无法加载管理页状态")).toBeInTheDocument();
    expect(screen.getByText("bootstrap load failed")).toBeInTheDocument();
  });

  it("shows read-only app state and disables save controls", async () => {
    const app = makeApp({
      id: "bot-readonly",
      name: "Readonly Bot",
      readOnly: true,
      readOnlyReason: "当前由启动参数接管，只能查看状态，不能在管理页修改。",
    });

    installMockFetch({
      "/api/admin/bootstrap-state": { body: makeBootstrap() },
      "/api/admin/runtime-status": { body: makeRuntimeStatus() },
      "/api/admin/feishu/apps": { body: { apps: [app] } },
      "/api/admin/feishu/manifest": { body: { manifest: makeManifest() } },
      "/api/admin/vscode/detect": { body: makeVSCodeDetect() },
      "/api/admin/instances": { body: { instances: [] } },
      "/api/admin/storage/image-staging": { body: makeImageStagingStatus() },
      "/api/admin/storage/preview-drive/bot-readonly": {
        body: makePreviewDriveStatus({ gatewayId: "bot-readonly", name: "Readonly Bot" }),
      },
    });

    render(<AdminRoute />);

    expect(await screen.findAllByText("当前由启动参数接管，只能查看状态，不能在管理页修改。")).not.toHaveLength(0);
    expect(screen.getByLabelText("机器人名称")).toBeDisabled();
    expect(screen.getByRole("button", { name: "保存更改" })).toBeDisabled();
  });
});
