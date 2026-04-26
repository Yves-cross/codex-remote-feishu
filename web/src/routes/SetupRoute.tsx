import { useEffect, useState, type ReactNode } from "react";
import {
  APIRequestError,
  type APIErrorShape,
  type JSONResult,
  requestJSON,
  requestJSONAllowHTTPError,
  requestVoid,
  sendJSON,
} from "../lib/api";
import { relativeLocalPath } from "../lib/paths";
import type {
  BootstrapState,
  FeishuAppResponse,
  FeishuAppSummary,
  FeishuAppTestStartResponse,
  FeishuAppVerifyResponse,
  FeishuManifestResponse,
  FeishuOnboardingCompleteResponse,
  FeishuOnboardingSession,
  FeishuOnboardingSessionResponse,
  OnboardingWorkflowAppStep,
  OnboardingWorkflowMachineStep,
  OnboardingWorkflowPermission,
  OnboardingWorkflowResponse,
  OnboardingWorkflowStage,
  SetupCompleteResponse,
} from "../lib/types";
import {
  blankToUndefined,
  buildSetupFeishuVerifySuccessMessage,
  vscodeApplyModeForScenario,
  vscodeIsReady,
} from "./shared/helpers";

type NoticeTone = "good" | "warn" | "danger";

type Notice = {
  tone: NoticeTone;
  message: string;
};

type ManualConnectForm = {
  name: string;
  appId: string;
  appSecret: string;
};

type TestState = {
  status: "idle" | "sending" | "sent" | "error";
  message: string;
};

type RuntimeApplyFailureDetails = {
  gatewayId?: string;
  app?: FeishuAppSummary;
};

type RequirementTableRow = {
  key: string;
  cells: ReactNode[];
};

const defaultQRCodePollIntervalSeconds = 5;
const vscodeApplyTimeoutMs = 10_000;
const vscodeDetectRecoveryTimeoutMs = 5_000;

export function SetupRoute() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [manifest, setManifest] = useState<FeishuManifestResponse["manifest"] | null>(
    null,
  );
  const [workflow, setWorkflow] = useState<OnboardingWorkflowResponse | null>(null);
  const [visibleStageID, setVisibleStageID] = useState("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [connectMode, setConnectMode] = useState<"qr" | "manual">("qr");
  const [manualForm, setManualForm] = useState<ManualConnectForm>({
    name: "",
    appId: "",
    appSecret: "",
  });
  const [actionBusy, setActionBusy] = useState("");
  const [onboardingSession, setOnboardingSession] =
    useState<FeishuOnboardingSession | null>(null);
  const [connectError, setConnectError] = useState("");
  const [eventTest, setEventTest] = useState<TestState>({
    status: "idle",
    message: "",
  });
  const [callbackTest, setCallbackTest] = useState<TestState>({
    status: "idle",
    message: "",
  });

  const title = buildSetupPageTitle(bootstrap);
  const adminURL = relativeLocalPath(bootstrap?.admin.url || "/");
  const currentStageID = workflow?.currentStage || workflow?.stages[0]?.id || "runtime_requirements";
  const stageID = visibleStageID || currentStageID;
  const currentStage = workflowStageByID(workflow, currentStageID);
  const activeStage = workflowStageByID(workflow, stageID);
  const activeApp = workflow?.app?.app ?? null;
  const activeConsoleLinks = activeApp?.consoleLinks;
  const isReadOnlyApp = Boolean(activeApp?.readOnly);
  const connectionStage = workflow?.app?.connection || workflowStageByID(workflow, "connect");
  const permissionStage = workflow?.app?.permission || null;
  const eventsStage = workflow?.app?.events || null;
  const callbackStage = workflow?.app?.callback || null;
  const menuStage = workflow?.app?.menu || null;

  useEffect(() => {
    document.title = title;
  }, [title]);

  useEffect(() => {
    let cancelled = false;
    void loadSetupPage({ focusCurrentStage: true }).catch(() => {
      if (!cancelled) {
        setLoadError("当前页面暂时无法读取状态，请刷新后重试。");
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    if (!activeApp) {
      setManualForm({ name: "", appId: "", appSecret: "" });
      return;
    }
    setManualForm((current) => ({
      name: current.name || activeApp.name || "",
      appId: current.appId || activeApp.appId || "",
      appSecret: current.appSecret,
    }));
  }, [activeApp?.id, activeApp?.name, activeApp?.appId]);

  useEffect(() => {
    setEventTest({ status: "idle", message: "" });
    setCallbackTest({ status: "idle", message: "" });
  }, [activeApp?.id]);

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
    if (typeof window.scrollTo === "function") {
      window.scrollTo({ top: 0, behavior: "auto" });
    }
  }, [stageID]);

  useEffect(() => {
    if (currentStageID === "connect") {
      return;
    }
    setOnboardingSession(null);
    setConnectError("");
  }, [currentStageID]);

  useEffect(() => {
    if (currentStageID !== "connect" || connectMode !== "qr") {
      return;
    }
    if (!stageAllowsAction(connectionStage, "start_qr")) {
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
  }, [actionBusy, connectError, connectMode, connectionStage, currentStageID, onboardingSession]);

  useEffect(() => {
    if (
      currentStageID !== "events" ||
      !activeApp?.id ||
      !stageAllowsAction(eventsStage, "start_test") ||
      eventTest.status !== "idle"
    ) {
      return;
    }
    void startTest(activeApp.id, "events");
  }, [activeApp?.id, currentStageID, eventTest.status, eventsStage]);

  useEffect(() => {
    if (
      currentStageID !== "callback" ||
      !activeApp?.id ||
      !stageAllowsAction(callbackStage, "start_test") ||
      callbackTest.status !== "idle"
    ) {
      return;
    }
    void startTest(activeApp.id, "callback");
  }, [activeApp?.id, callbackStage, callbackTest.status, currentStageID]);

  async function loadSetupPage(options?: {
    preferredAppID?: string;
    soft?: boolean;
    focusCurrentStage?: boolean;
  }) {
    if (!options?.soft) {
      setLoading(true);
    }
    setLoadError("");
    const preferredAppID =
      options?.preferredAppID || workflow?.selectedAppId || activeApp?.id || "";
    const [bootstrapState, manifestState, workflowState] = await Promise.all([
      requestJSON<BootstrapState>("/api/setup/bootstrap-state"),
      requestJSON<FeishuManifestResponse>("/api/setup/feishu/manifest"),
      requestJSON<OnboardingWorkflowResponse>(buildWorkflowPath(preferredAppID)),
    ]);

    setBootstrap(bootstrapState);
    setManifest(manifestState.manifest);
    setWorkflow(workflowState);
    if (options?.focusCurrentStage) {
      setVisibleStageID(workflowState.currentStage || workflowState.stages[0]?.id || "");
    }
    setLoading(false);
    return workflowState;
  }

  async function retryEnvironmentCheck() {
    await loadSetupPage({
      preferredAppID: activeApp?.id,
      soft: true,
      focusCurrentStage: true,
    });
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
        "/api/setup/feishu/onboarding/sessions",
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
        `/api/setup/feishu/onboarding/sessions/${encodeURIComponent(sessionID)}`,
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
        `/api/setup/feishu/onboarding/sessions/${encodeURIComponent(sessionID)}/complete`,
        { method: "POST" },
      );
      setOnboardingSession(response.data.session);
      if (!response.ok) {
        setConnectError("扫码已经完成，但连接验证没有通过，请重新验证。");
        return;
      }
      await loadSetupPage({
        preferredAppID: response.data.app.id,
        soft: true,
        focusCurrentStage: true,
      });
      setNotice({
        tone: "good",
        message: buildSetupFeishuVerifySuccessMessage(
          response.data.app,
          response.data.mutation,
        ),
      });
      setConnectError("");
    } catch {
      setConnectError("扫码已经完成，但当前还不能继续，请稍后重试。");
    } finally {
      setActionBusy("");
    }
  }

  async function submitManualConnect() {
    if (!activeApp && !manualForm.appId.trim()) {
      setNotice({ tone: "danger", message: "请填写完整的 App ID 和 App Secret。" });
      return;
    }
    if (!isReadOnlyApp && (!manualForm.appId.trim() || !manualForm.appSecret.trim())) {
      setNotice({ tone: "danger", message: "请填写完整的 App ID 和 App Secret。" });
      return;
    }

    setActionBusy("manual-connect");
    setNotice(null);
    try {
      let appID = activeApp?.id || "";
      if (!isReadOnlyApp) {
        const payload = {
          name: blankToUndefined(manualForm.name),
          appId: blankToUndefined(manualForm.appId),
          appSecret: blankToUndefined(manualForm.appSecret),
          enabled: true,
        };
        const saved = activeApp?.id
          ? await sendJSON<FeishuAppResponse>(
              `/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}`,
              "PUT",
              payload,
            )
          : await sendJSON<FeishuAppResponse>("/api/setup/feishu/apps", "POST", payload);
        appID = saved.app.id;
      }
      const verify = await requestJSONAllowHTTPError<FeishuAppVerifyResponse>(
        `/api/setup/feishu/apps/${encodeURIComponent(appID)}/verify`,
        { method: "POST" },
      );
      await loadSetupPage({
        preferredAppID: appID,
        soft: true,
        focusCurrentStage: true,
      });
      if (!verify.ok) {
        setNotice({
          tone: "danger",
          message: "连接验证没有通过，请检查 App ID 和 App Secret 后重试。",
        });
        return;
      }
      setNotice({
        tone: "good",
        message: buildSetupFeishuVerifySuccessMessage(verify.data.app),
      });
    } catch (error: unknown) {
      if (await maybeRecoverRuntimeApplyFailure(error, activeApp?.id)) {
        return;
      }
      setNotice({ tone: "danger", message: "当前还不能完成连接，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function maybeRecoverRuntimeApplyFailure(
    error: unknown,
    fallbackAppID?: string,
  ): Promise<boolean> {
    if (!(error instanceof APIRequestError) || error.code !== "gateway_apply_failed") {
      return false;
    }
    const details = error.details as RuntimeApplyFailureDetails | undefined;
    await loadSetupPage({
      preferredAppID: details?.app?.id || details?.gatewayId || fallbackAppID,
      soft: true,
      focusCurrentStage: true,
    });
    setNotice({
      tone: "warn",
      message:
        "配置已经保存，但当前运行中的机器人还没有同步完成。你可以稍后去管理页面继续处理。",
    });
    return true;
  }

  async function refreshWorkflowFocus() {
    await loadSetupPage({
      preferredAppID: activeApp?.id,
      soft: true,
      focusCurrentStage: true,
    });
  }

  async function startTest(appID: string, kind: "events" | "callback") {
    const setState = kind === "events" ? setEventTest : setCallbackTest;
    setState({ status: "sending", message: "" });
    const response = await requestJSONAllowHTTPError<FeishuAppTestStartResponse | APIErrorShape>(
      `/api/setup/feishu/apps/${encodeURIComponent(appID)}/${kind === "events" ? "test-events" : "test-callback"}`,
      {
        method: "POST",
      },
    );
    if (!response.ok) {
      const error = readAPIError(response);
      setState({
        status: "error",
        message:
          error?.code === "feishu_app_web_test_recipient_unavailable"
            ? String(
                error.details ||
                  "手动添加的机器人无法自动发送测试消息，请直接在飞书后台继续手动配置。",
              )
            : "暂时没有把测试提示发送成功，请稍后重试。",
      });
      return;
    }
    setState({
      status: "sent",
      message: response.data.message,
    });
  }

  async function clearInstallTest(appID: string, kind: "events" | "callback") {
    await requestJSONAllowHTTPError<unknown>(
      `/api/setup/feishu/apps/${encodeURIComponent(appID)}/install-tests/${encodeURIComponent(kind)}/clear`,
      {
        method: "POST",
      },
    );
  }

  async function confirmAppStep(step: "events" | "callback" | "menu") {
    if (!activeApp?.id) {
      return;
    }
    setActionBusy(`confirm-${step}`);
    try {
      await requestVoid(
        `/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/onboarding-steps/${encodeURIComponent(step)}/complete`,
        {
          method: "POST",
        },
      );
      if (step === "events" || step === "callback") {
        await clearInstallTest(activeApp.id, step);
      }
      if (step === "events") {
        setEventTest({ status: "idle", message: "" });
      }
      if (step === "callback") {
        setCallbackTest({ status: "idle", message: "" });
      }
      await refreshWorkflowFocus();
      setNotice({ tone: "good", message: `${stepTitle(step)}已记录完成。` });
    } catch {
      setNotice({ tone: "danger", message: `当前还不能记录${stepTitle(step)}完成，请稍后重试。` });
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
      await requestVoid(`/api/setup/onboarding/machine-decisions/${kind}`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({ decision }),
      });
      await refreshWorkflowFocus();
      setNotice({ tone: "good", message });
    } catch {
      setNotice({ tone: "danger", message: "当前还不能保存你的选择，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function applyAutostart() {
    setActionBusy("autostart-apply");
    try {
      await sendJSON("/api/setup/autostart/apply", "POST");
      await refreshWorkflowFocus();
      setNotice({ tone: "good", message: "已启用自动启动。" });
    } catch {
      setNotice({ tone: "danger", message: "当前还不能启用自动启动，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  async function applyVSCode() {
    const vscode = workflow?.vscode.vscode || null;
    if (!vscode) {
      setNotice({ tone: "danger", message: "暂时还不能完成 VS Code 集成，请稍后重试。" });
      return;
    }
    setActionBusy("vscode-apply");
    try {
      const mode = vscodeApplyModeForScenario(vscode, "current_machine");
      await sendJSON(
        "/api/setup/vscode/apply",
        "POST",
        {
          mode: mode || "managed_shim",
          bundleEntrypoint: vscode.latestBundleEntrypoint,
        },
        { timeoutMs: vscodeApplyTimeoutMs },
      );
      await refreshWorkflowFocus();
      setNotice({ tone: "good", message: "VS Code 集成已完成。" });
    } catch (error: unknown) {
      if (await maybeRecoverVSCodeApply(error)) {
        return;
      }
      setNotice({
        tone: "danger",
        message: "当前还不能确认 VS Code 集成结果，请稍后重试。",
      });
    } finally {
      setActionBusy("");
    }
  }

  async function maybeRecoverVSCodeApply(error: unknown): Promise<boolean> {
    try {
      const refreshed = await loadSetupPage({
        preferredAppID: activeApp?.id,
        soft: true,
        focusCurrentStage: true,
      });
      const ready =
        refreshed.vscode.status === "complete" ||
        vscodeIsReady(refreshed.vscode.vscode || null);
      if (ready) {
        setNotice({ tone: "good", message: "VS Code 集成已完成。" });
        return true;
      }
    } catch {
      // ignore refresh failure and continue with timeout handling below
    }

    if (error instanceof APIRequestError && error.code === "request_timeout") {
      setNotice({
        tone: "warn",
        message: "集成请求返回超时，当前还不能确认已完成，请稍后重试。",
      });
      return true;
    }

    return false;
  }

  async function completeSetup() {
    setActionBusy("complete-setup");
    try {
      const response = await requestJSONAllowHTTPError<SetupCompleteResponse>(
        "/api/setup/complete",
        { method: "POST" },
      );
      if (!response.ok) {
        const error = readAPIError(response);
        setNotice({
          tone: "danger",
          message:
            typeof error?.details === "string" && error.details.trim()
              ? String(error.details)
              : "当前 setup 还不能完成，请先处理阻塞项。",
        });
        await refreshWorkflowFocus();
        return;
      }
      const nextURL = relativeLocalPath(response.data.adminURL || bootstrap?.admin.url || "/");
      window.location.assign(nextURL);
    } catch {
      setNotice({ tone: "danger", message: "当前还不能完成 setup，请稍后重试。" });
    } finally {
      setActionBusy("");
    }
  }

  function renderWorkflowOverview() {
    if (!workflow) {
      return null;
    }
    const remainingActions = workflow.guide?.remainingManualActions || [];
    return (
      <div className="detail-stack">
        <div
          className={`notice-banner ${
            workflow.completion.canComplete ? "good" : "warn"
          }`}
        >
          {workflow.completion.summary}
        </div>
        {workflow.guide?.autoConfiguredSummary ? (
          <p className="support-copy">{workflow.guide.autoConfiguredSummary}</p>
        ) : null}
        {currentStage?.title ? (
          <p className="support-copy">当前推荐处理：{currentStage.title}</p>
        ) : null}
        {!workflow.completion.canComplete && workflow.completion.blockingReason ? (
          <div className="notice-banner warn">{workflow.completion.blockingReason}</div>
        ) : null}
        {remainingActions.length > 0 ? (
          <div className="panel">
            <div className="section-heading">
              <div>
                <h4>剩余建议项</h4>
                <p>这些项目不会都阻塞 setup 完成，但页面会继续按 workflow 提示你处理。</p>
              </div>
            </div>
            <ul className="ordered-checklist">
              {remainingActions.map((item) => (
                <li key={item}>{item}</li>
              ))}
            </ul>
          </div>
        ) : null}
        {workflow.completion.canComplete && stageID !== "done" ? (
          <div className="button-row">
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "complete-setup"}
              onClick={() => void completeSetup()}
            >
              完成设置并进入管理页面
            </button>
          </div>
        ) : null}
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
      case "done":
        return renderDoneStage();
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
          <p>当前页面直接使用后端 workflow 返回的环境检查结果。</p>
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
        {activeApp ? (
          <div className="notice-banner good">
            当前应用：{activeApp.name || activeApp.id}
            {activeApp.readOnly ? "（运行时接管，只能验证，不可在网页修改）" : ""}
          </div>
        ) : null}
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
        {connectMode === "qr" ? renderQRCodePanel() : renderManualPanel()}
      </section>
    );
  }

  function renderQRCodePanel() {
    const canStart = stageAllowsAction(connectionStage, "start_qr");
    return (
      <div className="panel">
        <div className="scan-preview">
          <div>
            <h4 style={{ margin: 0 }}>扫码创建</h4>
            <p className="support-copy">
              请使用飞书扫码完成创建，页面会按当前 workflow 自动刷新连接阶段。
            </p>
            <div className="scan-frame">
              {onboardingSession?.qrCodeDataUrl ? (
                <img alt="飞书扫码创建二维码" src={onboardingSession.qrCodeDataUrl} />
              ) : (
                <span>{canStart ? "二维码准备中" : "当前暂时不能发起扫码"}</span>
              )}
            </div>
          </div>
          <div className="detail-stack">
            {onboardingSession?.status === "pending" ? (
              <div className="notice-banner warn">正在等待扫码结果...</div>
            ) : null}
            {onboardingSession?.status === "ready" && !connectError ? (
              <div className="notice-banner good">扫码成功，正在刷新当前连接状态...</div>
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
                  disabled={actionBusy === "qr-start" || !canStart}
                  onClick={() => {
                    setOnboardingSession(null);
                    setConnectError("");
                  }}
                >
                  重新扫码
                </button>
              )}
              {onboardingSession?.status === "ready" && connectError ? (
                <button
                  className="secondary-button"
                  type="button"
                  disabled={actionBusy === "qr-complete"}
                  onClick={() => {
                    if (onboardingSession?.id) {
                      setConnectError("");
                      void completeQRCodeSession(onboardingSession.id);
                    }
                  }}
                >
                  重新验证
                </button>
              ) : null}
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
    );
  }

  function renderManualPanel() {
    const canSubmit = stageAllowsAction(connectionStage, "submit_manual");
    return (
      <div className="panel">
        {isReadOnlyApp ? (
          <div className="notice-banner warn">
            当前机器人信息由当前运行环境提供，网页里不能修改，只能完成连接验证。
          </div>
        ) : null}
        <div className="form-grid">
          <label className="field">
            <span>
              App ID <em className="field-required">*</em>
            </span>
            <input
              aria-label="App ID"
              disabled={isReadOnlyApp}
              placeholder="请输入 App ID"
              value={manualForm.appId}
              onChange={(event) =>
                setManualForm((current) => ({
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
              disabled={isReadOnlyApp}
              placeholder="请输入 App Secret"
              value={manualForm.appSecret}
              onChange={(event) =>
                setManualForm((current) => ({
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
              disabled={isReadOnlyApp}
              placeholder="例如：团队机器人"
              value={manualForm.name}
              onChange={(event) =>
                setManualForm((current) => ({
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
            disabled={actionBusy === "manual-connect" || !canSubmit}
            onClick={() => void submitManualConnect()}
          >
            验证并继续
          </button>
        </div>
      </div>
    );
  }

  function renderPermissionStage() {
    if (!permissionStage) {
      return renderUnavailableStage("权限检查", "当前还没有可用的飞书应用，暂时无法检查权限。");
    }
    const showWarning = permissionStage.status !== "complete";
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>权限检查</h2>
          <p>{permissionStage.summary}</p>
        </div>
        <div className={`notice-banner ${showWarning ? "warn" : "good"}`}>
          {permissionStage.status === "complete"
            ? "当前基础权限已经齐全。"
            : "这一步现在是建议补齐项，不会单独决定 setup 是否可完成。"}
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
              <button
                className="ghost-button"
                type="button"
                onClick={() => void copyGrantJSON(permissionStage.grantJSON || "")}
              >
                复制配置
              </button>
              {stageAllowsAction(permissionStage, "open_auth") ? (
                <a
                  className="ghost-button"
                  href={activeConsoleLinks?.auth || "#"}
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
    if (!eventsStage) {
      return renderUnavailableStage("事件订阅", "当前还没有可用的飞书应用，暂时无法进入事件订阅联调。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>事件订阅</h2>
          <p>{eventsStage.summary}</p>
        </div>
        {eventTest.status === "sent" ? (
          <div className="notice-banner good">
            {eventTest.message || "事件订阅测试提示已发送。"}
          </div>
        ) : null}
        {eventTest.status === "error" ? (
          <div className="notice-banner danger">{eventTest.message}</div>
        ) : null}
        <p className="support-copy">
          前往{" "}
          <a
            className="inline-link"
            href={activeConsoleLinks?.events || "#"}
            rel="noreferrer"
            target="_blank"
          >
            飞书后台
          </a>{" "}
          配置事件订阅。
        </p>
        {renderRequirementTable(
          ["事件", "用途"],
          (manifest?.events || []).map((item) => ({
            key: item.event,
            cells: [
              renderCopyableRequirement(item.event, "事件名"),
              item.purpose || "",
            ],
          })),
        )}
        <div className="button-row">
          {stageAllowsAction(eventsStage, "start_test") ? (
            <button
              className="secondary-button"
              type="button"
              disabled={actionBusy === "test-events" || !activeApp?.id}
              onClick={() => activeApp?.id && void startTest(activeApp.id, "events")}
            >
              重新发送测试提示
            </button>
          ) : null}
          {stageAllowsAction(eventsStage, "confirm") ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "confirm-events"}
              onClick={() => void confirmAppStep("events")}
            >
              我已完成，继续
            </button>
          ) : null}
        </div>
      </section>
    );
  }

  function renderCallbackStage() {
    if (!callbackStage) {
      return renderUnavailableStage("回调配置", "当前还没有可用的飞书应用，暂时无法进入回调联调。");
    }
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>回调配置</h2>
          <p>{callbackStage.summary}</p>
        </div>
        {callbackTest.status === "sent" ? (
          <div className="notice-banner good">
            {callbackTest.message || "回调测试卡片已发送。"}
          </div>
        ) : null}
        {callbackTest.status === "error" ? (
          <div className="notice-banner danger">{callbackTest.message}</div>
        ) : null}
        <p className="support-copy">
          前往{" "}
          <a
            className="inline-link"
            href={activeConsoleLinks?.callback || "#"}
            rel="noreferrer"
            target="_blank"
          >
            飞书后台
          </a>{" "}
          配置回调。
        </p>
        {renderRequirementTable(
          ["回调", "用途"],
          (manifest?.callbacks || []).map((item) => ({
            key: item.callback,
            cells: [
              renderCopyableRequirement(item.callback, "回调名"),
              item.purpose || "",
            ],
          })),
        )}
        <div className="button-row">
          {stageAllowsAction(callbackStage, "start_test") ? (
            <button
              className="secondary-button"
              type="button"
              disabled={actionBusy === "test-callback" || !activeApp?.id}
              onClick={() => activeApp?.id && void startTest(activeApp.id, "callback")}
            >
              重新发送测试提示
            </button>
          ) : null}
          {stageAllowsAction(callbackStage, "confirm") ? (
            <button
              className="primary-button"
              type="button"
              disabled={actionBusy === "confirm-callback"}
              onClick={() => void confirmAppStep("callback")}
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
      return renderUnavailableStage("菜单确认", "当前还没有可用的飞书应用，暂时无法确认菜单。");
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
            href={activeConsoleLinks?.bot || "#"}
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

  function renderDoneStage() {
    return (
      <section className="step-section">
        <div className="step-stage-head">
          <h2>欢迎使用</h2>
          <p>当前 workflow 已经允许完成 setup。</p>
        </div>
        <div className="completed-card">
          <h3>欢迎，设置已经完成。</h3>
          <p>你现在可以进入管理页面，继续维护机器人、系统集成和存储清理。</p>
        </div>
        <div className="button-row">
          <button
            className="primary-button"
            type="button"
            disabled={actionBusy === "complete-setup"}
            onClick={() => void completeSetup()}
          >
            完成设置并进入管理页面
          </button>
          <a className="ghost-button" href={adminURL}>
            直接查看管理页面
          </a>
        </div>
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

  async function copyGrantJSON(value: string) {
    if (!value.trim()) {
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
      setNotice({ tone: "good", message: "已复制权限配置。" });
    } catch {
      setNotice({ tone: "warn", message: "复制没有成功，请手动复制。" });
    }
  }

  async function copyRequirementValue(value: string, label: string) {
    if (!value.trim()) {
      return;
    }
    try {
      await navigator.clipboard.writeText(value);
      setNotice({ tone: "good", message: `已复制${label}。` });
    } catch {
      setNotice({ tone: "warn", message: `${label}复制没有成功，请手动复制。` });
    }
  }

  function renderCopyableRequirement(value: string, label: string) {
    return (
      <div className="requirement-copy-cell">
        <code>{value}</code>
        <button
          className="table-copy-button"
          type="button"
          aria-label={`复制${label} ${value}`}
          onClick={() => void copyRequirementValue(value, label)}
        >
          复制
        </button>
      </div>
    );
  }

  if (loading) {
    return (
      <div className="product-page">
        <header className="product-topbar">
          <h1>{title}</h1>
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
          <h1>{title}</h1>
        </header>
        <section className="panel">
          <div className="empty-state error">
            <strong>当前页面暂时无法打开</strong>
            <p>{loadError}</p>
            <div className="button-row">
              <button
                className="secondary-button"
                type="button"
                onClick={() => void loadSetupPage({ focusCurrentStage: true })}
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
        <h1>{title}</h1>
      </header>
      {notice ? (
        <div className="product-notice-slot">
          <div className={`notice-banner ${notice.tone}`}>{notice.message}</div>
        </div>
      ) : null}
      <main className="setup-grid">
        <aside className="panel step-rail">
          <div className="step-stage-head">
            <h2>设置流程</h2>
            <p>当前步骤和状态全部来自后端 workflow。</p>
          </div>
          <div className="step-list">
            {(workflow?.stages || []).map((stage) => (
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
          {renderCurrentStage()}
        </section>
      </main>
    </div>
  );
}

function buildWorkflowPath(preferredAppID: string): string {
  if (!preferredAppID.trim()) {
    return "/api/setup/onboarding/workflow";
  }
  return `/api/setup/onboarding/workflow?app=${encodeURIComponent(preferredAppID)}`;
}

function stepTitle(step: "events" | "callback" | "menu"): string {
  switch (step) {
    case "events":
      return "事件订阅";
    case "callback":
      return "回调配置";
    case "menu":
      return "菜单确认";
    default:
      return step;
  }
}

function workflowStageByID(
  workflow: OnboardingWorkflowResponse | null,
  id: string,
): OnboardingWorkflowStage | null {
  if (!workflow) {
    return null;
  }
  return workflow.stages.find((stage) => stage.id === id) || null;
}

function stageAllowsAction(
  stage:
    | OnboardingWorkflowStage
    | OnboardingWorkflowPermission
    | OnboardingWorkflowAppStep
    | OnboardingWorkflowMachineStep
    | null
    | undefined,
  action: string,
): boolean {
  if (!stage) {
    return false;
  }
  return (stage.allowedActions || []).includes(action);
}

function isResolvedStageStatus(status: string): boolean {
  return status === "complete" || status === "deferred" || status === "not_applicable";
}

function workflowStageLabel(stage: OnboardingWorkflowStage, currentStageID: string): string {
  if (stage.id === currentStageID) {
    return "当前推荐";
  }
  if (isResolvedStageStatus(stage.status)) {
    return "已处理";
  }
  if (stage.status === "blocked") {
    return "阻塞";
  }
  return "待处理";
}

function buildSetupPageTitle(bootstrap: BootstrapState | null): string {
  const name = bootstrap?.product.name?.trim() || "Codex Remote Feishu";
  const version = bootstrap?.product.version?.trim();
  return version ? `${name} ${version} 安装程序` : `${name} 安装程序`;
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

function readAPIError(result: JSONResult<unknown>) {
  if (result.ok) {
    return null;
  }
  const payload = result.data as APIErrorShape;
  return payload.error || null;
}
