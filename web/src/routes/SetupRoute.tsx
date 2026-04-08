import { useEffect, useMemo, useState } from "react";
import { formatError, requestJSON, requestJSONAllowHTTPError, sendJSON } from "../lib/api";
import type {
  BootstrapState,
  FeishuAppMutation,
  FeishuAppPublishCheckResponse,
  FeishuAppResponse,
  FeishuAppSummary,
  FeishuAppVerifyResponse,
  FeishuAppsResponse,
  FeishuManifestResponse,
  SetupCompleteResponse,
  VSCodeDetectResponse,
} from "../lib/types";
import { BlockingModal, ErrorState, LoadingState, Panel, StatusBadge } from "../components/ui";
import {
  SetupStepContent,
  SetupStepPrimaryAction,
  SetupStepSecondaryAction,
} from "./setup/SetupStepContent";
import {
  appToDraft,
  chooseAppID,
  defaultStepFor,
  emptyDraft,
  isStepReachable,
  preferredSetupAppFromLocation,
  previousStepFor,
  stepState,
  stepStateLabel,
  stepStateTone,
} from "./setup/helpers";
import type { BlockingErrorState, SetupDraft, SetupNotice, StepID } from "./setup/types";
import { newAppID, wizardSteps } from "./setup/types";
import {
  blankToUndefined,
  buildSetupFeishuVerifySuccessMessage,
  loadVSCodeState,
  vscodeIsReady,
} from "./shared/helpers";

export function SetupRoute() {
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [apps, setApps] = useState<FeishuAppSummary[]>([]);
  const [manifest, setManifest] = useState<FeishuManifestResponse["manifest"] | null>(null);
  const [vscode, setVSCode] = useState<VSCodeDetectResponse | null>(null);
  const [vscodeError, setVSCodeError] = useState<string>("");
  const [selectedID, setSelectedID] = useState<string>(() => preferredSetupAppFromLocation());
  const [draft, setDraft] = useState<SetupDraft>(emptyDraft());
  const [setupStarted, setSetupStarted] = useState(false);
  const [permissionsConfirmed, setPermissionsConfirmed] = useState(false);
  const [eventsConfirmed, setEventsConfirmed] = useState(false);
  const [longConnectionConfirmed, setLongConnectionConfirmed] = useState(false);
  const [menusConfirmed, setMenusConfirmed] = useState(false);
  const [vscodeDeferred, setVSCodeDeferred] = useState(false);
  const [currentStepHint, setCurrentStepHint] = useState<StepID>("start");
  const [error, setError] = useState<string>("");
  const [notice, setNotice] = useState<SetupNotice | null>(null);
  const [busyAction, setBusyAction] = useState<string>("");
  const [finishInfo, setFinishInfo] = useState<SetupCompleteResponse | null>(null);
  const [blockingError, setBlockingError] = useState<BlockingErrorState>(null);

  async function loadData(preferredID?: string) {
    const [bootstrapState, appList, manifestResponse, vscodeState] = await Promise.all([
      requestJSON<BootstrapState>("/api/setup/bootstrap-state"),
      requestJSON<FeishuAppsResponse>("/api/setup/feishu/apps"),
      requestJSON<FeishuManifestResponse>("/api/setup/feishu/manifest"),
      loadVSCodeState("/api/setup/vscode/detect"),
    ]);
    const nextSelectedID = chooseAppID(appList.apps, preferredID ?? selectedID);
    const nextActiveApp = appList.apps.find((app) => app.id === nextSelectedID) ?? null;

    setBootstrap(bootstrapState);
    setApps(appList.apps);
    setManifest(manifestResponse.manifest);
    setVSCode(vscodeState.data);
    setVSCodeError(vscodeState.error);
    setSelectedID(nextSelectedID);
    setDraft(appToDraft(nextActiveApp));
    setCurrentStepHint((current) => {
      const fallback = defaultStepFor(bootstrapState, appList.apps, nextActiveApp, vscodeState.data, vscodeDeferred, setupStarted);
      if (current === "start" && fallback !== "start") {
        return fallback;
      }
      return isStepReachable(current, bootstrapState, nextActiveApp) ? current : fallback;
    });
  }

  useEffect(() => {
    let cancelled = false;
    void loadData()
      .then(() => {
        if (!cancelled) {
          setError("");
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(formatError(err));
        }
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const activeApp = useMemo(() => apps.find((app) => app.id === selectedID) ?? null, [apps, selectedID]);
  const scopesJSON = useMemo(() => JSON.stringify(manifest?.scopesImport ?? { scopes: { tenant: [], user: [] } }, null, 2), [manifest]);

  useEffect(() => {
    if (apps.length > 0) {
      setSetupStarted(true);
    }
  }, [apps.length]);

  useEffect(() => {
    setDraft(appToDraft(activeApp));
  }, [activeApp?.id, activeApp?.name, activeApp?.appId, activeApp?.hasSecret]);

  useEffect(() => {
    setPermissionsConfirmed(Boolean(activeApp?.wizard?.scopesExportedAt));
    setEventsConfirmed(Boolean(activeApp?.wizard?.eventsConfirmedAt));
    setLongConnectionConfirmed(Boolean(activeApp?.wizard?.callbacksConfirmedAt));
    setMenusConfirmed(Boolean(activeApp?.wizard?.menusConfirmedAt));
  }, [
    activeApp?.id,
    activeApp?.wizard?.scopesExportedAt,
    activeApp?.wizard?.eventsConfirmedAt,
    activeApp?.wizard?.callbacksConfirmedAt,
    activeApp?.wizard?.menusConfirmedAt,
  ]);

  const resolvedCurrentStep = useMemo(
    () => (isStepReachable(currentStepHint, bootstrap, activeApp) ? currentStepHint : defaultStepFor(bootstrap, apps, activeApp, vscode, vscodeDeferred, setupStarted)),
    [activeApp, apps, bootstrap, currentStepHint, setupStarted, vscode, vscodeDeferred],
  );
  const currentStepIndex = wizardSteps.findIndex((step) => step.id === resolvedCurrentStep);
  const currentStepMeta = wizardSteps[currentStepIndex >= 0 ? currentStepIndex : 0];
  const stepCompletion = {
    start: setupStarted || apps.length > 0,
    connect: Boolean(activeApp?.wizard?.connectionVerifiedAt),
    permissions: Boolean(activeApp?.wizard?.scopesExportedAt),
    events: Boolean(activeApp?.wizard?.eventsConfirmedAt),
    longConnection: Boolean(activeApp?.wizard?.callbacksConfirmedAt),
    menus: Boolean(activeApp?.wizard?.menusConfirmedAt),
    publish: Boolean(activeApp?.wizard?.publishedAt),
    vscode: vscodeDeferred || vscodeIsReady(vscode),
  };

  useEffect(() => {
    window.scrollTo({ top: 0, behavior: "auto" });
    document.documentElement.scrollTop = 0;
    document.body.scrollTop = 0;
  }, [resolvedCurrentStep]);

  async function runAction(label: string, work: () => Promise<void>) {
    setBusyAction(label);
    setNotice(null);
    try {
      await work();
    } catch (err: unknown) {
      showBlockingError("这一步还没有完成", formatError(err));
    } finally {
      setBusyAction("");
    }
  }

  function showBlockingError(title: string, message: string, detail?: string) {
    setBlockingError({ title, message, detail });
  }

  async function copyText(value: string, successMessage: string) {
    await runAction("copy-text", async () => {
      if (!navigator.clipboard?.writeText) {
        throw new Error("当前浏览器不支持复制到剪贴板。");
      }
      await navigator.clipboard.writeText(value);
      setNotice({ tone: "good", message: successMessage });
    });
  }

  async function testAndContinue() {
    const hasPersistedSecret = Boolean(activeApp?.hasSecret);
    if (activeApp?.readOnly) {
      await verifyExistingAppAndAdvance(activeApp.id);
      return;
    }
    if (draft.appId.trim() === "") {
      showBlockingError("这一步还没有完成", "请先填写 App ID。");
      return;
    }
    if (draft.appSecret.trim() === "" && !hasPersistedSecret) {
      showBlockingError("这一步还没有完成", "请先填写 App Secret。");
      return;
    }

    await runAction("connect-app", async () => {
      const payload = {
        name: blankToUndefined(draft.name),
        appId: blankToUndefined(draft.appId),
        appSecret: blankToUndefined(draft.appSecret),
        enabled: true,
      };
      const response = draft.isNew
        ? await sendJSON<FeishuAppResponse>("/api/setup/feishu/apps", "POST", payload)
        : await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(selectedID)}`, "PUT", payload);
      await verifyExistingAppAndAdvance(response.app.id, response.mutation);
    });
  }

  async function verifyExistingAppAndAdvance(appID: string, mutation?: FeishuAppMutation) {
    await runAction("verify-app", async () => {
      const response = await requestJSONAllowHTTPError<FeishuAppVerifyResponse>(`/api/setup/feishu/apps/${encodeURIComponent(appID)}/verify`, {
        method: "POST",
      });
      await loadData(appID);
      if (!response.ok) {
        const detail = `${response.data.result.errorCode || "verify_failed"} ${response.data.result.errorMessage || ""}`.trim();
        showBlockingError("这一步还没有完成", "飞书应用连接测试失败，请检查 App ID、App Secret，以及飞书平台里是否已经添加机器人能力。", detail);
        return;
      }
      setNotice({
        tone: mutation?.kind === "identity_changed" || response.data.app.status?.state !== "connected" ? "warn" : "good",
        message: buildSetupFeishuVerifySuccessMessage(response.data.app, mutation),
      });
      setSetupStarted(true);
      setCurrentStepHint("permissions");
    });
  }

  async function confirmPermissionsAndContinue() {
    if (!permissionsConfirmed) {
      showBlockingError("这一步还没有完成", "请先在飞书平台完成权限导入，并勾选页面上的确认项。");
      return;
    }
    if (!activeApp) {
      showBlockingError("这一步还没有完成", "当前还没有可用的飞书应用。");
      return;
    }
    await runAction("wizard-permissions", async () => {
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/wizard`, "PATCH", { scopesExported: true });
      await loadData(activeApp.id);
      setNotice({ tone: "good", message: "权限导入已记录，继续下一步。" });
      setCurrentStepHint("events");
    });
  }

  async function confirmEventsAndContinue() {
    if (!eventsConfirmed) {
      showBlockingError("这一步还没有完成", "请先在飞书平台完成事件订阅，并勾选页面上的确认项。");
      return;
    }
    if (!activeApp) {
      showBlockingError("这一步还没有完成", "当前还没有可用的飞书应用。");
      return;
    }
    await runAction("wizard-events", async () => {
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/wizard`, "PATCH", { eventsConfirmed: true });
      await loadData(activeApp.id);
      setNotice({ tone: "good", message: "事件订阅已记录，继续下一步。" });
      setCurrentStepHint("longConnection");
    });
  }

  async function confirmLongConnectionAndContinue() {
    if (!longConnectionConfirmed) {
      showBlockingError("这一步还没有完成", "请先在飞书平台把回调订阅方式保存为长连接，并勾选页面上的确认项。");
      return;
    }
    if (!activeApp) {
      showBlockingError("这一步还没有完成", "当前还没有可用的飞书应用。");
      return;
    }
    await runAction("wizard-long-connection", async () => {
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/wizard`, "PATCH", { callbacksConfirmed: true });
      await loadData(activeApp.id);
      setNotice({ tone: "good", message: "回调长连接配置已记录，继续下一步。" });
      setCurrentStepHint("menus");
    });
  }

  async function confirmMenusAndContinue() {
    if (!menusConfirmed) {
      showBlockingError("这一步还没有完成", "请先在飞书平台完成机器人菜单配置，并勾选页面上的确认项。");
      return;
    }
    if (!activeApp) {
      showBlockingError("这一步还没有完成", "当前还没有可用的飞书应用。");
      return;
    }
    await runAction("wizard-menus", async () => {
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/wizard`, "PATCH", { menusConfirmed: true });
      await loadData(activeApp.id);
      setNotice({ tone: "good", message: "机器人菜单配置已记录，继续下一步。" });
      setCurrentStepHint("publish");
    });
  }

  async function checkPublishAndContinue() {
    if (!activeApp) {
      showBlockingError("这一步还没有完成", "当前还没有可用的飞书应用。");
      return;
    }
    await runAction("publish-check", async () => {
      const response = await requestJSONAllowHTTPError<FeishuAppPublishCheckResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/publish-check`, {
        method: "POST",
      });
      await loadData(activeApp.id);
      if (!response.ok || !response.data.ready) {
        showBlockingError(
          "这一步还没有完成",
          "发布验收没有通过。请先回到飞书后台完成缺失项，再重新点击“检查并继续”。",
          (response.data.issues ?? []).join("\n"),
        );
        return;
      }
      setNotice({ tone: "good", message: "发布验收通过，继续下一步。" });
      setCurrentStepHint("vscode");
    });
  }

  async function applyRecommendedVSCode() {
    await runAction("vscode-apply", async () => {
      const response = await sendJSON<VSCodeDetectResponse>("/api/setup/vscode/apply", "POST", {
        mode: vscode?.recommendedMode || "all",
      });
      setVSCode(response);
      setVSCodeError("");
      setVSCodeDeferred(false);
      setNotice({ tone: "good", message: `VS Code 推荐模式已应用：${response.recommendedMode}。` });
      setCurrentStepHint("finish");
    });
  }

  async function finishSetup() {
    if (!bootstrap) {
      return;
    }
    await runAction("finish-setup", async () => {
      const response = await sendJSON<SetupCompleteResponse>("/api/setup/complete", "POST");
      if (bootstrap.session.trustedLoopback) {
        window.location.assign("/");
        return;
      }
      setFinishInfo(response);
      setNotice({ tone: "good", message: response.message });
    });
  }

  function goToPreviousStep() {
    const previous = previousStepFor(resolvedCurrentStep);
    if (previous) {
      setCurrentStepHint(previous);
    }
  }

  if (finishInfo && bootstrap && !bootstrap.session.trustedLoopback) {
    return (
      <div className="app-shell wizard-shell">
        <aside className="side-rail wizard-rail">
          <div className="brand-lockup">
            <div className="brand-mark">CR</div>
            <div>
              <p className="brand-kicker">Setup Completed</p>
              <h1>Codex Remote</h1>
            </div>
          </div>
          <p className="side-copy">当前 setup access 已关闭。远程 SSH 场景下，正式管理页仍然只允许 localhost 访问。</p>
        </aside>
        <main className="main-stage">
          <Panel title="安装向导已完成" description={finishInfo.message}>
            <div className="wizard-link-row">
              <span>本地管理页地址</span>
              <a href={finishInfo.adminURL} target="_blank" rel="noreferrer">
                {finishInfo.adminURL}
              </a>
            </div>
          </Panel>
        </main>
      </div>
    );
  }

  return (
    <>
      <div className="app-shell wizard-shell">
        <aside className="side-rail wizard-rail">
          <div className="brand-lockup">
            <div className="brand-mark">CR</div>
            <div>
              <p className="brand-kicker">Setup Wizard</p>
              <h1>Codex Remote</h1>
            </div>
          </div>
          <p className="side-copy">向导一次只展示当前步骤。左侧只保留步骤名和状态，不提前暴露后面的配置细节。</p>
          <div className="wizard-step-nav" aria-label="Setup Steps">
            {wizardSteps.map((step) => {
              const state = stepState(step.id, resolvedCurrentStep, stepCompletion, bootstrap, activeApp);
              const disabled = state === "locked";
              return (
                <button key={step.id} type="button" className={`wizard-step-link${step.id === resolvedCurrentStep ? " current" : ""}`} onClick={() => setCurrentStepHint(step.id)} disabled={disabled}>
                  <div>
                    <strong>{step.label}</strong>
                    <p>{step.summary}</p>
                  </div>
                  <StatusBadge value={stepStateLabel(state)} tone={stepStateTone(state)} />
                </button>
              );
            })}
          </div>
        </aside>

        <main className="main-stage wizard-stage">
          <header className="page-hero wizard-hero">
            <div>
              <p className="page-kicker">
                Setup Step {currentStepIndex + 1}/{wizardSteps.length}
              </p>
              <h2>{currentStepMeta.label}</h2>
              <p className="wizard-hero-copy">{currentStepMeta.summary}</p>
            </div>
            <div className="hero-actions">
              <button className="secondary-button" type="button" onClick={() => void loadData(activeApp?.id)} disabled={busyAction !== ""}>
                刷新状态
              </button>
            </div>
          </header>

          {notice ? <div className={`notice-banner ${notice.tone}`}>{notice.message}</div> : null}
          {!bootstrap && !error ? <LoadingState title="正在初始化 Setup 页面" description="读取 bootstrap、飞书应用、manifest 和 VS Code 检测结果。" /> : null}
          {error ? <ErrorState title="无法加载 Setup 状态" description="setup shell 已就位，但当前状态读取失败。" detail={error} /> : null}
          {bootstrap && manifest ? (
            <Panel title={currentStepMeta.label} description={currentStepMeta.summary} className="wizard-panel">
              <SetupStepContent
                currentStep={resolvedCurrentStep}
                apps={apps}
                activeApp={activeApp}
                manifest={manifest}
                draft={draft}
                scopesJSON={scopesJSON}
                permissionsConfirmed={permissionsConfirmed}
                eventsConfirmed={eventsConfirmed}
                longConnectionConfirmed={longConnectionConfirmed}
                menusConfirmed={menusConfirmed}
                vscodeComplete={stepCompletion.vscode}
                vscode={vscode}
                vscodeError={vscodeError}
                onDraftChange={setDraft}
                onPermissionsConfirmedChange={setPermissionsConfirmed}
                onEventsConfirmedChange={setEventsConfirmed}
                onLongConnectionConfirmedChange={setLongConnectionConfirmed}
                onMenusConfirmedChange={setMenusConfirmed}
                onCopyScopes={() => void copyText(scopesJSON, "权限配置 JSON 已复制。")}
                busyAction={busyAction}
              />
              <div className="wizard-footer">
                <div className="wizard-footer-left">
                  {resolvedCurrentStep !== "start" ? (
                    <button className="ghost-button" type="button" onClick={goToPreviousStep} disabled={busyAction !== ""}>
                      上一步
                    </button>
                  ) : null}
                </div>
                <div className="wizard-footer-right">
                  <SetupStepSecondaryAction
                    currentStep={resolvedCurrentStep}
                    busyAction={busyAction}
                    onCopyScopes={() => void copyText(scopesJSON, "权限配置 JSON 已复制。")}
                    onDeferVSCode={() => {
                      setVSCodeDeferred(true);
                      setNotice({ tone: "warn", message: "VS Code 集成已留到本地管理页继续处理。" });
                      setCurrentStepHint("finish");
                    }}
                  />
                  <SetupStepPrimaryAction
                    currentStep={resolvedCurrentStep}
                    busyAction={busyAction}
                    canApplyVSCode={Boolean(vscode)}
                    onStart={() => {
                      setSetupStarted(true);
                      setCurrentStepHint("connect");
                    }}
                    onTestAndContinue={() => void testAndContinue()}
                    onConfirmPermissions={() => void confirmPermissionsAndContinue()}
                    onConfirmEvents={() => void confirmEventsAndContinue()}
                    onConfirmLongConnection={() => void confirmLongConnectionAndContinue()}
                    onConfirmMenus={() => void confirmMenusAndContinue()}
                    onCheckPublish={() => void checkPublishAndContinue()}
                    onApplyRecommendedVSCode={() => void applyRecommendedVSCode()}
                    onFinishSetup={() => void finishSetup()}
                  />
                </div>
              </div>
            </Panel>
          ) : null}
        </main>
      </div>

      <BlockingModal open={Boolean(blockingError)} title={blockingError?.title || ""} message={blockingError?.message || ""} detail={blockingError?.detail} onConfirm={() => setBlockingError(null)} />
    </>
  );
}
