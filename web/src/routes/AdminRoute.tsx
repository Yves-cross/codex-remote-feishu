import { useEffect, useMemo, useState, type ReactNode } from "react";
import {
  APIRequestError,
  type APIErrorShape,
  requestJSON,
  requestJSONAllowHTTPError,
  requestVoid,
  sendJSON,
} from "../lib/api";
import type {
  BootstrapState,
  FeishuAppResponse,
  FeishuAppSummary,
  FeishuAppTestStartResponse,
  FeishuAppVerifyResponse,
  FeishuAppsResponse,
  FeishuManifestResponse,
  FeishuOnboardingCompleteResponse,
  FeishuOnboardingSession,
  FeishuOnboardingSessionResponse,
  ImageStagingCleanupResponse,
  ImageStagingStatusResponse,
  LogsStorageCleanupResponse,
  LogsStorageStatusResponse,
  OnboardingWorkflowAppStep,
  OnboardingWorkflowMachineStep,
  OnboardingWorkflowPermission,
  OnboardingWorkflowResponse,
  OnboardingWorkflowStage,
  PreviewDriveCleanupResponse,
  PreviewDriveStatusResponse,
} from "../lib/types";
import {
  blankToUndefined,
  buildAdminFeishuVerifySuccessMessage,
  vscodeApplyModeForScenario,
  vscodeIsReady,
} from "./shared/helpers";
import {
  buildOnboardingWorkflowPath,
  isResolvedStageStatus,
  onboardingStepTitle,
  stageAllowsAction,
  workflowStageByID,
  workflowStageLabel,
} from "./shared/onboardingWorkflow";

type NoticeTone = "good" | "warn" | "danger";

type DetailNotice = {
  tone: NoticeTone;
  message: string;
};

type RuntimeApplyFailureDetails = {
  gatewayId?: string;
  app?: FeishuAppSummary;
};

type NewRobotForm = {
  name: string;
  appId: string;
  appSecret: string;
};

type RequirementTableRow = {
  key: string;
  cells: ReactNode[];
};

const newRobotID = "new";
const defaultQRCodePollIntervalSeconds = 5;

export function AdminRoute() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [manifest, setManifest] = useState<FeishuManifestResponse["manifest"] | null>(
    null,
  );
  const [apps, setApps] = useState<FeishuAppSummary[]>([]);
  const [selectedRobotID, setSelectedRobotID] = useState(newRobotID);
  const [detailNotice, setDetailNotice] = useState<DetailNotice | null>(null);
  const [newRobotForm, setNewRobotForm] = useState<NewRobotForm>({
    name: "",
    appId: "",
    appSecret: "",
  });
  const [connectMode, setConnectMode] = useState<"qr" | "manual">("qr");
  const [onboardingSession, setOnboardingSession] =
    useState<FeishuOnboardingSession | null>(null);
  const [connectError, setConnectError] = useState("");
  const [workflowLoading, setWorkflowLoading] = useState(false);
  const [workflowError, setWorkflowError] = useState("");
  const [workflow, setWorkflow] = useState<OnboardingWorkflowResponse | null>(null);
  const [visibleStageID, setVisibleStageID] = useState("");
  const [imageStaging, setImageStaging] =
    useState<ImageStagingStatusResponse | null>(null);
  const [imageStagingError, setImageStagingError] = useState("");
  const [logsStorage, setLogsStorage] = useState<LogsStorageStatusResponse | null>(
    null,
  );
  const [logsStorageError, setLogsStorageError] = useState("");
  const [previewMap, setPreviewMap] = useState<
    Record<string, PreviewDriveStatusResponse>
  >({});
  const [previewError, setPreviewError] = useState("");
  const [actionBusy, setActionBusy] = useState("");
  const [deleteTargetID, setDeleteTargetID] = useState<string | null>(null);

  const selectedApp = useMemo(
    () => apps.find((app) => app.id === selectedRobotID) ?? null,
    [apps, selectedRobotID],
  );
  const versionTitle = buildAdminPageTitle(bootstrap);
  const previewSummary = useMemo(() => {
    return Object.values(previewMap).reduce(
      (accumulator, item) => {
        accumulator.fileCount += item.summary.fileCount;
        accumulator.bytes += item.summary.estimatedBytes;
        return accumulator;
      },
      { fileCount: 0, bytes: 0 },
    );
  }, [previewMap]);

  const currentStageID =
    workflow?.currentStage || workflow?.stages[0]?.id || "runtime_requirements";
  const stageID = visibleStageID || currentStageID;
  const currentStage = workflowStageByID(workflow, currentStageID);
  const activeStage = workflowStageByID(workflow, stageID);
  const connectionStage =
    workflow?.app?.connection || workflowStageByID(workflow, "connect");
  const permissionStage = workflow?.app?.permission || null;
  const eventsStage = workflow?.app?.events || null;
  const callbackStage = workflow?.app?.callback || null;
  const menuStage = workflow?.app?.menu || null;

  useEffect(() => {
    document.title = versionTitle;
  }, [versionTitle]);

  useEffect(() => {
    void loadAdminPage().catch(() => {
      setLoadError("当前页面暂时无法读取状态，请刷新后重试。");
      setLoading(false);
    });
  }, []);

  useEffect(() => {
    if (selectedRobotID === newRobotID) {
      setWorkflow(null);
      setWorkflowError("");
      setWorkflowLoading(false);
      setVisibleStageID("");
      return;
    }
    void loadWorkflow(selectedRobotID, true);
  }, [selectedRobotID]);

  useEffect(() => {
    if (!workflow) {
      return;
    }
    if (visibleStageID && workflow.stages.some((stage) => stage.id === visibleStageID)) {
      return;
    }
    setVisibleStageID(workflow.currentStage || workflow.stages[0]?.id || "");
  }, [visibleStageID, workflow]);

  useEffect(() => {
    if (selectedRobotID !== newRobotID || connectMode !== "qr") {
      return;
    }
    if (actionBusy === "qr-start" || actionBusy === "qr-complete") {
      return;
    }
    if (!onboardingSession) {
      if (!connectError) {
        void startQRCodeSession();
      }
      return;
    }
    if (onboardingSession.status === "ready" && !connectError) {
      void completeQRCodeSession(onboardingSession.id);
      return;
    }
    if (onboardingSession.status !== "pending") {
      return;
    }
    const pollDelaySeconds = Math.max(
      onboardingSession.pollIntervalSeconds || defaultQRCodePollIntervalSeconds,
      defaultQRCodePollIntervalSeconds,
    );
    const timer = window.setTimeout(() => {
      void refreshQRCodeSession(onboardingSession.id);
    }, pollDelaySeconds * 1_000);
    return () => window.clearTimeout(timer);
  }, [actionBusy, connectError, connectMode, onboardingSession, selectedRobotID]);

  async function loadAdminPage(options?: { preferredRobotID?: string }) {
    setLoading(true);
    setLoadError("");

    const [bootstrapState, manifestState, appList, imageResult, logsResult] =
      await Promise.all([
        requestJSON<BootstrapState>("/api/admin/bootstrap-state"),
        requestJSON<FeishuManifestResponse>("/api/admin/feishu/manifest"),
        requestJSON<FeishuAppsResponse>("/api/admin/feishu/apps"),
        safeRequest<ImageStagingStatusResponse>("/api/admin/storage/image-staging"),
        safeRequest<LogsStorageStatusResponse>("/api/admin/storage/logs"),
      ]);

    const previewResults = await Promise.allSettled(
      appList.apps.map(async (app) => {
        const data = await requestJSON<PreviewDriveStatusResponse>(
          `/api/admin/storage/preview-drive/${encodeURIComponent(app.id)}`,
        );
        return [app.id, data] as const;
      }),
    );

    const previews: Record<string, PreviewDriveStatusResponse> = {};
    let previewFailed = false;
    previewResults.forEach((result) => {
      if (result.status === "fulfilled") {
        previews[result.value[0]] = result.value[1];
        return;
      }
      previewFailed = true;
    });

    const nextSelectedRobotID =
      appList.apps.find((app) => app.id === options?.preferredRobotID)?.id ||
      appList.apps.find((app) => app.id === selectedRobotID)?.id ||
      appList.apps[0]?.id ||
      newRobotID;

    setBootstrap(bootstrapState);
    setManifest(manifestState.manifest);
    setApps(appList.apps);
    setSelectedRobotID(nextSelectedRobotID);
    setImageStaging(imageResult.data);
    setImageStagingError(imageResult.error);
    setLogsStorage(logsResult.data);
    setLogsStorageError(logsResult.error);
    setPreviewMap(previews);
    setPreviewError(previewFailed ? "部分预览文件状态暂时没有读取成功。" : "");
    setLoading(false);
  }

  async function loadWorkflow(preferredRobotID: string, focusCurrentStage: boolean) {
    setWorkflowLoading(true);
    setWorkflowError("");
    try {
      const payload = await requestJSON<OnboardingWorkflowResponse>(
        buildOnboardingWorkflowPath("/api/admin", preferredRobotID),
      );
      setWorkflow(payload);
      if (focusCurrentStage) {
        setVisibleStageID(payload.currentStage || payload.stages[0]?.id || "");
      }
    } catch {
      setWorkflowError("当前 onboarding 状态暂时读取失败，请稍后重试。");
    } finally {
      setWorkflowLoading(false);
    }
  }

  function changeConnectMode(nextMode: "qr" | "manual") {
    setConnectMode(nextMode);
    setConnectError("");
    setOnboardingSession(null);
  }

  async function startQRCodeSession() {
    setActionBusy("qr-start");
    setConnectError("");
    try {
      const response = await sendJSON<FeishuOnboardingSessionResponse>(
        "/api/admin/feishu/onboarding/sessions",
        "POST",
      );
      setOnboardingSession(response.session);
    } catch {
      setConnectError("暂时无法开始扫码，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  async function refreshQRCodeSession(sessionID: string) {
    try {
      const response = await requestJSON<FeishuOnboardingSessionResponse>(
        `/api/admin/feishu/onboarding/sessions/${encodeURIComponent(sessionID)}`,
      );
      setOnboardingSession(response.session);
      if (response.session.status === "pending") {
        setConnectError("");
      }
    } catch {
      setConnectError("扫码状态暂时没有刷新成功，请稍后重试。");
    }
  }

  async function completeQRCodeSession(sessionID: string) {
    setActionBusy("qr-complete");
    try {
      const response = await requestJSONAllowHTTPError<FeishuOnboardingCompleteResponse>(
        `/api/admin/feishu/onboarding/sessions/${encodeURIComponent(sessionID)}/complete`,
        { method: "POST" },
      );
      setOnboardingSession(response.data.session);
      if (!response.ok) {
        setConnectError("扫码已经完成，但连接验证没有通过，请重新验证。");
        return;
      }
      await loadAdminPage({ preferredRobotID: response.data.app.id });
      setSelectedRobotID(response.data.app.id);
      setDetailNotice({
        tone: "good",
        message: buildAdminFeishuVerifySuccessMessage(
          response.data.app,
          response.data.result.duration,
        ),
      });
      setConnectError("");
      setOnboardingSession(null);
    } catch {
      setConnectError("扫码已经完成，但当前还不能继续，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  async function createRobot() {
    if (!newRobotForm.appId.trim() || !newRobotForm.appSecret.trim()) {
      setDetailNotice({
        tone: "danger",
        message: "请填写完整的 App ID 和 App Secret。",
      });
      return;
    }

    setActionBusy("create-robot");
    try {
      const saved = await sendJSON<FeishuAppResponse>("/api/admin/feishu/apps", "POST", {
        name: blankToUndefined(newRobotForm.name),
        appId: blankToUndefined(newRobotForm.appId),
        appSecret: blankToUndefined(newRobotForm.appSecret),
        enabled: true,
      });
      const verify = await requestJSONAllowHTTPError<FeishuAppVerifyResponse>(
        `/api/admin/feishu/apps/${encodeURIComponent(saved.app.id)}/verify`,
        { method: "POST" },
      );
      await loadAdminPage({ preferredRobotID: saved.app.id });
      setSelectedRobotID(saved.app.id);
      if (!verify.ok) {
        setDetailNotice({
          tone: "danger",
          message: "连接验证没有通过，请检查 App ID 和 App Secret 后重试。",
        });
        return;
      }
      setDetailNotice({
        tone: "good",
        message: buildAdminFeishuVerifySuccessMessage(
          verify.data.app,
          verify.data.result.duration,
        ),
      });
      setNewRobotForm({ name: "", appId: "", appSecret: "" });
    } catch (error: unknown) {
      if (await maybeRecoverRuntimeApplyFailure(error)) {
        return;
      }
      setDetailNotice({ tone: "danger", message: "当前还不能保存这个机器人，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function maybeRecoverRuntimeApplyFailure(error: unknown): Promise<boolean> {
    if (!(error instanceof APIRequestError) || error.code !== "gateway_apply_failed") {
      return false;
    }
    const details = error.details as RuntimeApplyFailureDetails | undefined;
    await loadAdminPage({
      preferredRobotID: details?.app?.id || details?.gatewayId,
    });
    if (details?.app?.id || details?.gatewayId) {
      setSelectedRobotID(details?.app?.id || details?.gatewayId || newRobotID);
    }
    setDetailNotice({
      tone: "warn",
      message:
        "配置已经保存，但当前运行中的机器人还没有同步完成。请稍后刷新状态后再继续。",
    });
    return true;
  }

  async function refreshWorkflowFocus() {
    if (!selectedApp?.id) {
      return;
    }
    await loadWorkflow(selectedApp.id, true);
  }

  async function retryEnvironmentCheck() {
    await refreshWorkflowFocus();
  }

  async function triggerRobotTest(kind: "events" | "callback") {
    if (!selectedApp?.id) {
      return;
    }
    setActionBusy(`test-${kind}`);
    const response = await requestJSONAllowHTTPError<FeishuAppTestStartResponse | APIErrorShape>(
      `/api/admin/feishu/apps/${encodeURIComponent(selectedApp.id)}/${kind === "events" ? "test-events" : "test-callback"}`,
      {
        method: "POST",
      },
    );
    if (!response.ok) {
      const payload = readAPIError(response);
      setDetailNotice({
        tone: "danger",
        message:
          payload?.code === "feishu_app_web_test_recipient_unavailable"
            ? String(
                payload.details ||
                  "手动添加的机器人无法自动发送测试消息，请直接在飞书后台继续手动配置。",
              )
            : kind === "events"
              ? "事件订阅测试没有发出，请稍后重试。"
              : "回调测试没有发出，请稍后重试。",
      });
      setActionBusy("");
      return;
    }
    setDetailNotice({
      tone: "good",
      message: (response.data as FeishuAppTestStartResponse).message,
    });
    setActionBusy("");
  }

  async function confirmAppStep(step: "events" | "callback" | "menu") {
    if (!selectedApp?.id) {
      return;
    }
    setActionBusy(`confirm-${step}`);
    try {
      await requestVoid(
        `/api/admin/feishu/apps/${encodeURIComponent(selectedApp.id)}/onboarding-steps/${encodeURIComponent(step)}/complete`,
        {
          method: "POST",
        },
      );
      await refreshWorkflowFocus();
      setDetailNotice({
        tone: "good",
        message: `${onboardingStepTitle(step)}已记录完成。`,
      });
    } catch {
      setDetailNotice({
        tone: "danger",
        message: `当前还不能记录${onboardingStepTitle(step)}完成，请稍后重试。`,
      });
    } finally {
      setActionBusy("");
    }
  }

  async function recordMachineDecision(
    kind: "autostart" | "vscode",
    decision: string,
    message: string,
  ) {
    setActionBusy(`${kind}-${decision}`);
    try {
      await requestVoid(`/api/admin/onboarding/machine-decisions/${kind}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ decision }),
      });
      await refreshWorkflowFocus();
      setDetailNotice({ tone: "good", message });
    } catch {
      setDetailNotice({ tone: "danger", message: "当前还不能保存你的选择，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function applyAutostart() {
    setActionBusy("autostart-apply");
    try {
      await sendJSON("/api/admin/autostart/apply", "POST");
      await refreshWorkflowFocus();
      setDetailNotice({ tone: "good", message: "已启用自动启动。" });
    } catch {
      setDetailNotice({ tone: "danger", message: "当前还不能启用自动启动，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function applyVSCode() {
    const vscode = workflow?.vscode.vscode || null;
    if (!vscode) {
      setDetailNotice({ tone: "danger", message: "暂时还不能完成 VS Code 集成，请稍后重试。" });
      return;
    }
    setActionBusy("vscode-apply");
    try {
      const mode = vscodeApplyModeForScenario(vscode, "current_machine");
      await sendJSON(
        "/api/admin/vscode/apply",
        "POST",
        {
          mode: mode || "managed_shim",
          bundleEntrypoint: vscode.latestBundleEntrypoint,
        },
      );
      await refreshWorkflowFocus();
      setDetailNotice({ tone: "good", message: "VS Code 集成已完成。" });
    } catch {
      setDetailNotice({
        tone: "danger",
        message: "当前还不能确认 VS Code 集成结果，请稍后重试。",
      });
    } finally {
      setActionBusy("");
    }
  }

  async function deleteRobot() {
    if (!deleteTargetID) {
      return;
    }
    setActionBusy("delete-robot");
    try {
      const response = await requestJSONAllowHTTPError<unknown>(
        `/api/admin/feishu/apps/${encodeURIComponent(deleteTargetID)}`,
        { method: "DELETE" },
      );
      if (!response.ok) {
        throw new APIRequestError(
          response.status,
          "delete failed",
          readAPIError(response)?.code,
          readAPIError(response)?.details,
        );
      }
      await loadAdminPage();
      setDetailNotice({ tone: "good", message: "机器人已删除。" });
      setDeleteTargetID(null);
    } catch (error: unknown) {
      if (await maybeRecoverRuntimeApplyFailure(error)) {
        setDeleteTargetID(null);
        return;
      }
      setDetailNotice({ tone: "danger", message: "当前还不能删除这个机器人，请稍后重试。" });
    } finally {
      setActionBusy("");
      setDeleteTargetID(null);
    }
  }

  async function cleanupImageStaging() {
    setActionBusy("cleanup-image");
    try {
      const response = await sendJSON<ImageStagingCleanupResponse>(
        "/api/admin/storage/image-staging/cleanup",
        "POST",
      );
      setImageStaging((current) =>
        current
          ? {
              ...current,
              fileCount: response.remainingFileCount,
              totalBytes: response.remainingBytes,
            }
          : current,
      );
      setImageStagingError("");
    } catch {
      setImageStagingError("图片暂存清理没有完成，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  async function cleanupLogsStorage() {
    setActionBusy("cleanup-logs");
    try {
      const response = await sendJSON<LogsStorageCleanupResponse>(
        "/api/admin/storage/logs/cleanup",
        "POST",
        { olderThanHours: 24 },
      );
      setLogsStorage((current) =>
        current
          ? {
              ...current,
              fileCount: response.remainingFileCount,
              totalBytes: response.remainingBytes,
            }
          : current,
      );
      setLogsStorageError("");
    } catch {
      setLogsStorageError("日志清理没有完成，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  async function cleanupPreviewDrive() {
    if (apps.length === 0) {
      return;
    }
    setActionBusy("cleanup-preview");
    try {
      const results = await Promise.allSettled(
        apps.map((app) =>
          sendJSON<PreviewDriveCleanupResponse>(
            `/api/admin/storage/preview-drive/${encodeURIComponent(app.id)}/cleanup`,
            "POST",
          ),
        ),
      );
      const nextMap: Record<string, PreviewDriveStatusResponse> = { ...previewMap };
      let failed = false;
      results.forEach((result) => {
        if (result.status !== "fulfilled") {
          failed = true;
          return;
        }
        nextMap[result.value.gatewayId] = {
          gatewayId: result.value.gatewayId,
          name: result.value.name,
          summary: result.value.result.summary,
        };
      });
      setPreviewMap(nextMap);
      setPreviewError(failed ? "部分预览文件暂时没有清理成功。" : "");
    } catch {
      setPreviewError("预览文件清理没有完成，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  function renderNewRobotDetail() {
    return (
      <section className="panel">
        <div className="step-stage-head">
          <h2>新增机器人</h2>
          <p>选择扫码创建或手动输入，连接验证通过后会直接切到 workflow-driven 详情区。</p>
        </div>
        <div className="choice-toggle">
          <button
            className={connectMode === "qr" ? "primary-button" : "ghost-button"}
            type="button"
            onClick={() => changeConnectMode("qr")}
          >
            扫码创建
          </button>
          <button
            className={connectMode === "manual" ? "primary-button" : "ghost-button"}
            type="button"
            onClick={() => changeConnectMode("manual")}
          >
            手动输入
          </button>
        </div>
        {connectMode === "qr" ? (
          <div className="panel">
            <div className="scan-preview">
              <div>
                <h4 style={{ margin: 0 }}>扫码创建</h4>
                <p className="support-copy">
                  请使用飞书扫码完成创建，页面会自动轮询并切换到新的机器人详情。
                </p>
                <div className="scan-frame">
                  {onboardingSession?.qrCodeDataUrl ? (
                    <img alt="飞书扫码创建二维码" src={onboardingSession.qrCodeDataUrl} />
                  ) : (
                    <span>二维码准备中</span>
                  )}
                </div>
              </div>
              <div className="detail-stack">
                {onboardingSession?.status === "pending" ? (
                  <div className="notice-banner warn">正在等待扫码结果...</div>
                ) : null}
                {onboardingSession?.status === "ready" && !connectError ? (
                  <div className="notice-banner good">
                    扫码成功，连接验证已通过，正在切换到机器人详情...
                  </div>
                ) : null}
                {onboardingSession?.status === "failed" ||
                onboardingSession?.status === "expired" ||
                connectError ? (
                  <div className="notice-banner danger">
                    {connectError || "当前扫码没有继续成功，请重新开始。"}
                  </div>
                ) : null}
                <div className="button-row">
                  {(connectError ||
                    onboardingSession?.status === "failed" ||
                    onboardingSession?.status === "expired") && (
                    <button
                      className="secondary-button"
                      type="button"
                      disabled={actionBusy === "qr-start"}
                      onClick={() => {
                        setOnboardingSession(null);
                        setConnectError("");
                      }}
                    >
                      重新扫码
                    </button>
                  )}
                  <button
                    className="ghost-button"
                    type="button"
                    onClick={() => changeConnectMode("manual")}
                  >
                    改用手动输入
                  </button>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <div className="panel">
            <div className="form-grid">
              <label className="field">
                <span>
                  App ID <em className="field-required">*</em>
                </span>
                <input
                  aria-label="App ID"
                  placeholder="请输入 App ID"
                  value={newRobotForm.appId}
                  onChange={(event) =>
                    setNewRobotForm((current) => ({
                      ...current,
                      appId: event.target.value,
                    }))
                  }
                />
              </label>
              <label className="field">
                <span>
                  App Secret <em className="field-required">*</em>
                </span>
                <input
                  aria-label="App Secret"
                  placeholder="请输入 App Secret"
                  value={newRobotForm.appSecret}
                  onChange={(event) =>
                    setNewRobotForm((current) => ({
                      ...current,
                      appSecret: event.target.value,
                    }))
                  }
                />
              </label>
              <label className="field form-grid-span-2">
                <span>机器人名称（可选）</span>
                <input
                  aria-label="机器人名称（可选）"
                  placeholder="例如：运营机器人"
                  value={newRobotForm.name}
                  onChange={(event) =>
                    setNewRobotForm((current) => ({
                      ...current,
                      name: event.target.value,
                    }))
                  }
                />
              </label>
            </div>
            <div className="button-row">
              <button
                className="primary-button"
                type="button"
                disabled={actionBusy === "create-robot"}
                onClick={() => void createRobot()}
              >
                验证并保存
              </button>
            </div>
          </div>
        )}
      </section>
    );
  }

  function renderWorkflowOverview() {
    if (!workflow) {
      return null;
    }
    const remainingActions = workflow.guide?.remainingManualActions || [];
    return (
      <div className="detail-stack">
        <div className="notice-banner warn">{workflow.completion.summary}</div>
        {workflow.guide?.autoConfiguredSummary ? (
          <p className="support-copy">{workflow.guide.autoConfiguredSummary}</p>
        ) : null}
        {currentStage?.title ? (
          <p className="support-copy">当前推荐处理：{currentStage.title}</p>
        ) : null}
        {workflow.completion.blockingReason ? (
          <div className="notice-banner warn">{workflow.completion.blockingReason}</div>
        ) : null}
        {remainingActions.length > 0 ? (
          <div className="panel">
            <div className="section-heading">
              <div>
                <h4>剩余建议项</h4>
                <p>这些项目会继续按 workflow 语义展示，不再由 AdminRoute 本地拼装。</p>
              </div>
            </div>
            <ul className="ordered-checklist">
              {remainingActions.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
        ) : null}
      </div>
    );
  }

  function renderWorkflowDetail() {
    if (workflowLoading) {
      return (
        <section className="panel">
          <div className="empty-state">
            <div className="loading-dot" />
            <span>正在读取 onboarding 状态</span>
          </div>
        </section>
      );
    }
    if (workflowError) {
      return (
        <section className="panel">
          <div className="empty-state error">
            <strong>当前详情暂时无法打开</strong>
            <p>{workflowError}</p>
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                onClick={() => selectedApp?.id && void loadWorkflow(selectedApp.id, true)}
              >
                重新加载
              </button>
            </div>
          </div>
        </section>
      );
    }
    if (!workflow || !selectedApp) {
      return null;
    }
    return (
      <div className="setup-grid">
        <aside className="panel step-rail">
          <div className="step-stage-head">
            <h2>{selectedApp.name || "未命名机器人"}</h2>
            <p>当前详情直接消费统一 onboarding workflow。</p>
          </div>
          <div className="step-list">
            {workflow.stages.map((stage) => (
              <button
                key={stage.id}
                className={`step-item${stage.id === stageID ? " active" : ""}${
                  isResolvedStageStatus(stage.status) ? " done" : ""
                }`}
                type="button"
                onClick={() => setVisibleStageID(stage.id)}
              >
                <strong>{stage.title}</strong>
                <span>{workflowStageLabel(stage, currentStageID)}</span>
              </button>
            ))}
          </div>
        </aside>
        <section className="panel step-stage">
          {renderWorkflowOverview()}
          {!selectedApp.readOnly ? (
            <div className="button-row" style={{ marginBottom: "1rem" }}>
              <button
                className="danger-button"
                type="button"
                onClick={() => setDeleteTargetID(selectedApp.id)}
              >
                删除机器人
              </button>
            </div>
          ) : null}
          {renderCurrentStage()}
        </section>
      </div>
    );
  }

  function renderCurrentStage() {
    switch (stageID) {
      case "runtime_requirements":
        return renderEnvironmentStage();
      case "connect":
        return renderConnectStage();
      case "permission":
        return renderPermissionStage();
      case "events":
        return renderEventsStage();
      case "callback":
        return renderCallbackStage();
      case "menu":
        return renderMenuStage();
      case "autostart":
        return renderAutostartStage();
      case "vscode":
        return renderVSCodeStage();
      default:
        return renderEnvironmentStage();
    }
  }

  function renderEnvironmentStage() {
    const runtimeRequirements = workflow?.runtimeRequirements;
    const failingChecks =
      runtimeRequirements?.checks.filter((check) => check.status !== "pass") || [];
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>环境检查</h2>
          <p>当前详情直接使用后端 workflow 返回的环境检查结果。</p>
        </div>
        <div className={`notice-banner ${runtimeRequirements?.ready ? "good" : "warn"}`}>
          {runtimeRequirements?.summary || "当前服务还在检查中，请稍候。"}
        </div>
        {failingChecks.length > 0 ? (
          <div className="panel">
            <div className="section-heading">
              <div>
                <h4>当前需要处理</h4>
                <p>请先修复下面的问题，再重新检查。</p>
              </div>
            </div>
            <ul className="ordered-checklist">
              {failingChecks.map((check) => (
                <li key={check.id}>
                  <strong>{check.title}</strong>
                  <span>{check.summary}</span>
                </li>
              ))}
            </ul>
          </div>
        ) : null}
        <div className="button-row">
          <button
            className="secondary-button"
            type="button"
            onClick={() => void retryEnvironmentCheck()}
          >
            重新检查
          </button>
        </div>
      </section>
    );
  }

  function renderConnectStage() {
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>飞书连接</h2>
          <p>{connectionStage?.summary || "接入并验证一个可用的飞书应用。"}</p>
        </div>
        <div className="notice-banner good">
          当前应用：{selectedApp?.name || selectedApp?.id}
          {selectedApp?.readOnly ? "（运行时接管，只能验证，不可在网页修改）" : ""}
        </div>
        <div className="button-row">
          <button
            className="secondary-button"
            type="button"
            disabled={!stageAllowsAction(connectionStage, "verify") || !selectedApp?.id}
            onClick={() => selectedApp?.id && void verifyExistingRobot(selectedApp.id)}
          >
            重新验证
          </button>
        </div>
      </section>
    );
  }

  function renderPermissionStage() {
    if (!permissionStage) {
      return renderUnavailableStage("权限检查", "当前还没有可用的飞书应用，暂时无法检查权限。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>权限检查</h2>
          <p>{permissionStage.summary}</p>
        </div>
        <div className={`notice-banner ${permissionStage.status === "complete" ? "good" : "warn"}`}>
          {permissionStage.status === "complete"
            ? "当前基础权限已经齐全。"
            : "这一步现在是建议补齐项，不会单独决定 workflow 下一步由谁 owner。"}
        </div>
        {(permissionStage.missingScopes || []).length > 0 ? (
          <div className="scope-list">
            {(permissionStage.missingScopes || []).map((scope) => (
              <span
                key={`${scope.scopeType || "tenant"}-${scope.scope}`}
                className="scope-pill"
              >
                <code>{scope.scope}</code>
              </span>
            ))}
          </div>
        ) : null}
        {permissionStage.grantJSON ? (
          <div className="panel">
            <div className="section-heading">
              <div>
                <h4>可复制的一次性权限配置</h4>
                <p>补齐后刷新 workflow 即可看到最新状态。</p>
              </div>
            </div>
            <textarea
              readOnly
              className="code-textarea"
              value={permissionStage.grantJSON || ""}
            />
            <div className="button-row">
              {stageAllowsAction(permissionStage, "open_auth") ? (
                <a
                  className="ghost-button"
                  href={selectedApp?.consoleLinks?.auth || "#"}
                  rel="noreferrer"
                  target="_blank"
                >
                  打开飞书后台权限配置
                </a>
              ) : null}
            </div>
          </div>
        ) : null}
        <div className="button-row">
          <button
            className="secondary-button"
            type="button"
            disabled={!stageAllowsAction(permissionStage, "recheck")}
            onClick={() => void refreshWorkflowFocus()}
          >
            重新检查
          </button>
        </div>
      </section>
    );
  }

  function renderEventsStage() {
    return renderAppTestStage(
      "events",
      "事件订阅",
      eventsStage,
      selectedApp?.consoleLinks?.events || "#",
      "飞书后台",
      ["事件", "用途"],
      (manifest?.events || []).map((item) => ({
        key: item.event,
        cells: [renderCodeCell(item.event), item.purpose || ""],
      })),
    );
  }

  function renderCallbackStage() {
    return renderAppTestStage(
      "callback",
      "回调配置",
      callbackStage,
      selectedApp?.consoleLinks?.callback || "#",
      "飞书后台",
      ["回调", "用途"],
      (manifest?.callbacks || []).map((item) => ({
        key: item.callback,
        cells: [renderCodeCell(item.callback), item.purpose || ""],
      })),
    );
  }

  function renderAppTestStage(
    kind: "events" | "callback",
    titleText: string,
    stage: OnboardingWorkflowAppStep | null,
    consoleURL: string,
    consoleLabel: string,
    headers: string[],
    rows: RequirementTableRow[],
  ) {
    if (!stage) {
      return renderUnavailableStage(titleText, "当前还没有可用的飞书应用。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>{titleText}</h2>
          <p>{stage.summary}</p>
        </div>
        <p className="support-copy">
          前往{" "}
          <a
            className="inline-link"
            href={consoleURL}
            rel="noreferrer"
            target="_blank"
          >
            {consoleLabel}
          </a>{" "}
          完成当前联调。
        </p>
        {renderRequirementTable(headers, rows)}
        <div className="button-row">
          {stageAllowsAction(stage, "start_test") ? (
            <button
              className="secondary-button"
              type="button"
              disabled={actionBusy === `test-${kind}` || !selectedApp?.id}
              onClick={() => void triggerRobotTest(kind)}
            >
              发送测试提示
            </button>
          ) : null}
          {stageAllowsAction(stage, "confirm") ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === `confirm-${kind}`}
              onClick={() => void confirmAppStep(kind)}
            >
              我已完成，继续
            </button>
          ) : null}
        </div>
      </section>
    );
  }

  function renderMenuStage() {
    if (!menuStage) {
      return renderUnavailableStage("菜单确认", "当前还没有可用的飞书应用。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>菜单确认</h2>
          <p>{menuStage.summary}</p>
        </div>
        <p className="support-copy">
          前往{" "}
          <a
            className="inline-link"
            href={selectedApp?.consoleLinks?.bot || "#"}
            rel="noreferrer"
            target="_blank"
          >
            飞书后台
          </a>{" "}
          完成菜单配置。
        </p>
        <div className="button-row">
          {stageAllowsAction(menuStage, "confirm") ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "confirm-menu"}
              onClick={() => void confirmAppStep("menu")}
            >
              我已完成，继续
            </button>
          ) : null}
        </div>
      </section>
    );
  }

  function renderAutostartStage() {
    const autostartStage = workflow?.autostart;
    const autostart = autostartStage?.autostart || null;
    if (!autostartStage) {
      return renderUnavailableStage("自动启动", "暂时没有拿到自动启动状态。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>自动启动</h2>
          <p>{autostartStage.summary}</p>
        </div>
        <div
          className={`notice-banner ${
            autostartStage.status === "complete" ? "good" : "warn"
          }`}
        >
          {autostartStage.summary}
        </div>
        {autostartStage.error ? (
          <div className="notice-banner warn">{autostartStage.error}</div>
        ) : null}
        <div className="button-row">
          {stageAllowsAction(autostartStage, "apply") && autostart?.canApply ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "autostart-apply"}
              onClick={() => void applyAutostart()}
            >
              启用自动启动
            </button>
          ) : null}
          {stageAllowsAction(autostartStage, "record_enabled") ? (
            <button
              className="secondary-button"
              type="button"
              disabled={actionBusy === "autostart-enabled"}
              onClick={() =>
                void recordMachineDecision(
                  "autostart",
                  "enabled",
                  "已记录自动启动决策。",
                )
              }
            >
              已启用，记录完成
            </button>
          ) : null}
          {stageAllowsAction(autostartStage, "defer") ? (
            <button
              className="ghost-button"
              type="button"
              disabled={actionBusy === "autostart-deferred"}
              onClick={() =>
                void recordMachineDecision(
                  "autostart",
                  "deferred",
                  "自动启动已留待稍后处理。",
                )
              }
            >
              稍后处理
            </button>
          ) : null}
        </div>
      </section>
    );
  }

  function renderVSCodeStage() {
    const vscodeStage = workflow?.vscode;
    const vscode = vscodeStage?.vscode || null;
    if (!vscodeStage) {
      return renderUnavailableStage("VS Code 集成", "暂时没有拿到 VS Code 集成状态。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>VS Code 集成</h2>
          <p>{vscodeStage.summary}</p>
        </div>
        <div
          className={`notice-banner ${
            vscodeStage.status === "complete" ? "good" : "warn"
          }`}
        >
          {vscodeStage.summary}
        </div>
        {vscodeStage.error ? (
          <div className="notice-banner warn">{vscodeStage.error}</div>
        ) : null}
        <div className="button-row">
          {stageAllowsAction(vscodeStage, "apply") ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "vscode-apply"}
              onClick={() => void applyVSCode()}
            >
              确认集成
            </button>
          ) : null}
          {stageAllowsAction(vscodeStage, "record_managed_shim") ? (
            <button
              className="secondary-button"
              type="button"
              disabled={actionBusy === "vscode-managed_shim"}
              onClick={() =>
                void recordMachineDecision(
                  "vscode",
                  "managed_shim",
                  "已记录 VS Code 集成决策。",
                )
              }
            >
              已处理，记录完成
            </button>
          ) : null}
          {stageAllowsAction(vscodeStage, "remote_only") ? (
            <button
              className="ghost-button"
              type="button"
              disabled={actionBusy === "vscode-remote_only"}
              onClick={() =>
                void recordMachineDecision(
                  "vscode",
                  "remote_only",
                  "VS Code 集成已留到目标 SSH 机器上处理。",
                )
              }
            >
              去目标 SSH 机器处理
            </button>
          ) : null}
          {stageAllowsAction(vscodeStage, "defer") ? (
            <button
              className="ghost-button"
              type="button"
              disabled={actionBusy === "vscode-deferred"}
              onClick={() =>
                void recordMachineDecision(
                  "vscode",
                  "deferred",
                  "VS Code 集成已留待稍后处理。",
                )
              }
            >
              稍后处理
            </button>
          ) : null}
        </div>
        {vscode ? (
          <p className="support-copy">
            当前检测结果：{vscodeIsReady(vscode) ? "已接入" : "尚未接入"}。
          </p>
        ) : null}
      </section>
    );
  }

  function renderUnavailableStage(titleText: string, message: string) {
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>{titleText}</h2>
          <p>{message}</p>
        </div>
        <div className="notice-banner warn">{message}</div>
      </section>
    );
  }

  async function verifyExistingRobot(appID: string) {
    setActionBusy("verify");
    try {
      const response = await requestJSONAllowHTTPError<FeishuAppVerifyResponse>(
        `/api/admin/feishu/apps/${encodeURIComponent(appID)}/verify`,
        { method: "POST" },
      );
      await loadAdminPage({ preferredRobotID: appID });
      if (!response.ok) {
        setDetailNotice({
          tone: "danger",
          message: "连接验证没有通过，请检查当前配置后重试。",
        });
        return;
      }
      setDetailNotice({
        tone: "good",
        message: buildAdminFeishuVerifySuccessMessage(
          response.data.app,
          response.data.result.duration,
        ),
      });
    } finally {
      setActionBusy("");
    }
  }

  function renderCodeCell(value: string) {
    return (
      <div className="requirement-copy-cell">
        <code>{value}</code>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="product-page">
        <header className="product-topbar">
          <h1>{versionTitle}</h1>
          <p>管理机器人、预览文件与本地存储。</p>
        </header>
        <section className="panel">
          <div className="empty-state">
            <div className="loading-dot" />
            <span>正在读取最新状态</span>
          </div>
        </section>
      </div>
    );
  }

  if (loadError) {
    return (
      <div className="product-page">
        <header className="product-topbar">
          <h1>{versionTitle}</h1>
          <p>管理机器人、预览文件与本地存储。</p>
        </header>
        <section className="panel">
          <div className="empty-state error">
            <strong>当前页面暂时无法打开</strong>
            <p>{loadError}</p>
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                onClick={() => void loadAdminPage()}
              >
                重新加载
              </button>
            </div>
          </div>
        </section>
      </div>
    );
  }

  return (
    <div className="product-page">
      <header className="product-topbar">
        <h1>{versionTitle}</h1>
        <p>管理机器人、统一 onboarding 流和本地存储。</p>
      </header>

      <section className="panel">
        <div className="step-stage-head">
          <h2>机器人管理</h2>
          <p>左侧选择机器人，右侧直接显示同一套 workflow-driven 接入与补齐流程。</p>
        </div>
        {detailNotice ? (
          <div className={`notice-banner ${detailNotice.tone}`}>{detailNotice.message}</div>
        ) : null}
        <div className="robot-layout" style={{ marginTop: "1rem" }}>
          <div className="robot-list">
            {apps.map((app) => (
              <button
                key={app.id}
                className={`robot-list-button${selectedRobotID === app.id ? " active" : ""}`}
                type="button"
                onClick={() => {
                  setDetailNotice(null);
                  setSelectedRobotID(app.id);
                }}
              >
                <div className="robot-list-head">
                  <strong>{app.name || "未命名机器人"}</strong>
                  {app.runtimeApply?.pending ? (
                    <span className="robot-tag warn">同步中</span>
                  ) : null}
                </div>
                <p>{app.appId || "未填写 App ID"}</p>
              </button>
            ))}
            <button
              className={`robot-list-button${selectedRobotID === newRobotID ? " active" : ""}`}
              type="button"
              onClick={() => {
                setDetailNotice(null);
                setSelectedRobotID(newRobotID);
              }}
            >
              <div className="robot-list-head">
                <strong>新增机器人</strong>
                <span className="robot-tag">新增</span>
              </div>
              <p>点击开始接入</p>
            </button>
          </div>
          {selectedRobotID === newRobotID ? renderNewRobotDetail() : renderWorkflowDetail()}
        </div>
      </section>

      <section className="panel">
        <div className="step-stage-head">
          <h2>存储管理</h2>
          <p>查看占用并按需清理旧文件。</p>
        </div>
        <div className="soft-grid" style={{ marginTop: "1rem" }}>
          <article className="soft-card-v2">
            <h4>预览文件</h4>
            <p>{formatFileSummary(previewSummary.fileCount, previewSummary.bytes)}</p>
            {previewError ? <div className="notice-banner warn">{previewError}</div> : null}
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                disabled={actionBusy === "cleanup-preview" || apps.length === 0}
                onClick={() => void cleanupPreviewDrive()}
              >
                清理旧预览
              </button>
            </div>
          </article>
          <article className="soft-card-v2">
            <h4>图片暂存</h4>
            <p>
              {formatFileSummary(imageStaging?.fileCount || 0, imageStaging?.totalBytes || 0)}
            </p>
            {imageStagingError ? (
              <div className="notice-banner warn">{imageStagingError}</div>
            ) : null}
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                disabled={actionBusy === "cleanup-image"}
                onClick={() => void cleanupImageStaging()}
              >
                清理旧图片
              </button>
            </div>
          </article>
          <article className="soft-card-v2">
            <h4>日志文件</h4>
            <p>
              {formatFileSummary(logsStorage?.fileCount || 0, logsStorage?.totalBytes || 0)}
            </p>
            {logsStorageError ? (
              <div className="notice-banner warn">{logsStorageError}</div>
            ) : null}
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                disabled={actionBusy === "cleanup-logs"}
                onClick={() => void cleanupLogsStorage()}
              >
                清理一天前日志
              </button>
            </div>
          </article>
        </div>
      </section>

      {deleteTargetID ? (
        <div className="modal-backdrop" role="presentation">
          <div
            className="modal-card"
            role="dialog"
            aria-modal="true"
            aria-labelledby="delete-robot-title"
          >
            <h3 id="delete-robot-title">确认删除机器人</h3>
            <p className="modal-copy">
              删除后将移除“
              {apps.find((app) => app.id === deleteTargetID)?.name || "当前机器人"}
              ”，此操作不可恢复。
            </p>
            <div className="modal-actions">
              <button
                className="ghost-button"
                type="button"
                onClick={() => setDeleteTargetID(null)}
              >
                取消
              </button>
              <button
                className="danger-button"
                type="button"
                disabled={actionBusy === "delete-robot"}
                onClick={() => void deleteRobot()}
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}

async function safeRequest<T>(path: string) {
  try {
    return {
      data: await requestJSON<T>(path),
      error: "",
    };
  } catch {
    return {
      data: null,
      error: "暂时没有读取成功，请稍后重试。",
    };
  }
}

function readAPIError(response: { ok: boolean; data: unknown }) {
  if (response.ok) {
    return null;
  }
  const payload = response.data as APIErrorShape;
  return payload.error || null;
}

function buildAdminPageTitle(bootstrap: BootstrapState | null): string {
  const name = bootstrap?.product.name?.trim() || "Codex Remote Feishu";
  const version = bootstrap?.product.version?.trim();
  return version ? `${name} ${version} 管理` : `${name} 管理`;
}

function renderRequirementTable(headers: string[], rows: RequirementTableRow[]) {
  return (
    <div className="detail-table-wrap">
      <table className="detail-table">
        <thead>
          <tr>
            {headers.map((header) => (
              <th key={header} scope="col">
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, rowIndex) => (
            <tr key={row.key || `${rowIndex}-row`}>
              {row.cells.map((value, cellIndex) => (
                <td key={`${rowIndex}-${cellIndex}`}>{value}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatBytes(value: number): string {
  if (value <= 0) {
    return "0 B";
  }
  const units = ["B", "KB", "MB", "GB", "TB"];
  let current = value;
  let index = 0;
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024;
    index += 1;
  }
  return `${current >= 100 || index === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[index]}`;
}

function formatFileSummary(fileCount: number, bytes: number): string {
  return `${fileCount} 个文件，约 ${formatBytes(bytes)}`;
}
