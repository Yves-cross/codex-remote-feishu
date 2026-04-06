import { useEffect, useMemo, useState } from "react";
import { formatError, requestJSON, requestJSONAllowHTTPError, requestVoid, sendJSON } from "../lib/api";
import type {
  BootstrapState,
  FeishuAppResponse,
  FeishuAppSummary,
  FeishuAppVerifyResponse,
  FeishuAppsResponse,
  FeishuManifestResponse,
  GatewayStatus,
  SetupCompleteResponse,
  VSCodeDetectResponse,
} from "../lib/types";
import { DataList, DefinitionList, ErrorState, LoadingState, Panel, ShellFrame, StatCard, StatGrid, StatusBadge } from "../components/ui";

const newAppID = "__new__";

type SetupDraft = {
  isNew: boolean;
  id: string;
  name: string;
  appId: string;
  appSecret: string;
  enabled: boolean;
};

type Notice = {
  tone: "good" | "warn" | "danger";
  message: string;
};

const emptyDraft = (): SetupDraft => ({
  isNew: true,
  id: "",
  name: "",
  appId: "",
  appSecret: "",
  enabled: true,
});

export function SetupRoute() {
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [apps, setApps] = useState<FeishuAppSummary[]>([]);
  const [manifest, setManifest] = useState<FeishuManifestResponse["manifest"] | null>(null);
  const [vscode, setVSCode] = useState<VSCodeDetectResponse | null>(null);
  const [vscodeError, setVSCodeError] = useState<string>("");
  const [selectedID, setSelectedID] = useState<string>(newAppID);
  const [draft, setDraft] = useState<SetupDraft>(emptyDraft);
  const [error, setError] = useState<string>("");
  const [notice, setNotice] = useState<Notice | null>(null);
  const [busyAction, setBusyAction] = useState<string>("");
  const [finishInfo, setFinishInfo] = useState<SetupCompleteResponse | null>(null);

  async function loadData(preferredID?: string) {
    const [bootstrapState, appList, manifestResponse, vscodeState] = await Promise.all([
      requestJSON<BootstrapState>("/api/setup/bootstrap-state"),
      requestJSON<FeishuAppsResponse>("/api/setup/feishu/apps"),
      requestJSON<FeishuManifestResponse>("/api/setup/feishu/manifest"),
      loadVSCodeState("/api/setup/vscode/detect"),
    ]);
    setBootstrap(bootstrapState);
    setApps(appList.apps);
    setManifest(manifestResponse.manifest);
    setVSCode(vscodeState.data);
    setVSCodeError(vscodeState.error);
    syncDraftSelection(appList.apps, preferredID ?? selectedID, setSelectedID, setDraft);
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

  const activeApp = selectedID === newAppID ? null : apps.find((app) => app.id === selectedID) ?? null;
  const scopesJSON = useMemo(() => JSON.stringify(manifest?.scopesImport ?? { scopes: { tenant: [], user: [] } }, null, 2), [manifest]);
  const verifiedApps = useMemo(() => apps.filter((app) => Boolean(app.wizard?.connectionVerifiedAt || app.verifiedAt)), [apps]);
  const finishDisabledReason = bootstrap?.setupRequired
    ? "先保存至少一个带完整凭证的飞书 App"
    : verifiedApps.length === 0
      ? "至少完成一个飞书 App 的连通性验证"
      : "";

  const wizardChecklistItems = activeApp
    ? [
        { label: "凭证已保存", done: Boolean(activeApp.wizard?.credentialsSavedAt), timestamp: activeApp.wizard?.credentialsSavedAt },
        { label: "连接已验证", done: Boolean(activeApp.wizard?.connectionVerifiedAt), timestamp: activeApp.wizard?.connectionVerifiedAt },
        { label: "Scopes 已导出", done: Boolean(activeApp.wizard?.scopesExportedAt), timestamp: activeApp.wizard?.scopesExportedAt },
        { label: "事件已确认", done: Boolean(activeApp.wizard?.eventsConfirmedAt), timestamp: activeApp.wizard?.eventsConfirmedAt },
        { label: "回调已确认", done: Boolean(activeApp.wizard?.callbacksConfirmedAt), timestamp: activeApp.wizard?.callbacksConfirmedAt },
        { label: "菜单已确认", done: Boolean(activeApp.wizard?.menusConfirmedAt), timestamp: activeApp.wizard?.menusConfirmedAt },
        { label: "机器人已发布", done: Boolean(activeApp.wizard?.publishedAt), timestamp: activeApp.wizard?.publishedAt },
      ]
    : [];

  async function runAction(label: string, work: () => Promise<void>) {
    setBusyAction(label);
    setNotice(null);
    try {
      await work();
    } catch (err: unknown) {
      setNotice({ tone: "danger", message: formatError(err) });
    } finally {
      setBusyAction("");
    }
  }

  function selectApp(app: FeishuAppSummary) {
    setSelectedID(app.id);
    setDraft(appToDraft(app));
    setNotice(null);
  }

  function beginNewApp() {
    setSelectedID(newAppID);
    setDraft(emptyDraft());
    setNotice(null);
  }

  async function saveApp() {
    await runAction(draft.isNew ? "create-app" : "save-app", async () => {
      const payload = {
        id: draft.isNew ? blankToUndefined(draft.id) : undefined,
        name: blankToUndefined(draft.name),
        appId: blankToUndefined(draft.appId),
        appSecret: blankToUndefined(draft.appSecret),
        enabled: draft.enabled,
      };
      const path = draft.isNew ? "/api/setup/feishu/apps" : `/api/setup/feishu/apps/${encodeURIComponent(selectedID)}`;
      const method = draft.isNew ? "POST" : "PUT";
      const response = await sendJSON<FeishuAppResponse>(path, method, payload);
      await loadData(response.app.id);
      setNotice({ tone: "good", message: draft.isNew ? "飞书 App 已创建并写入配置。" : "飞书 App 配置已更新。" });
    });
  }

  async function deleteApp() {
    if (!activeApp) {
      return;
    }
    if (!window.confirm(`删除飞书 App “${activeApp.name || activeApp.id}”？`)) {
      return;
    }
    await runAction("delete-app", async () => {
      await requestVoid(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}`, { method: "DELETE" });
      await loadData(newAppID);
      setNotice({ tone: "good", message: "飞书 App 已删除。" });
    });
  }

  async function verifyApp() {
    if (!activeApp) {
      return;
    }
    await runAction("verify-app", async () => {
      const response = await requestJSONAllowHTTPError<FeishuAppVerifyResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/verify`, {
        method: "POST",
      });
      await loadData(activeApp.id);
      if (response.ok) {
        setNotice({ tone: "good", message: `验证成功，用时 ${(response.data.result.duration / 1_000_000_000).toFixed(1)}s。` });
        return;
      }
      setNotice({
        tone: "danger",
        message: `验证失败：${response.data.result.errorCode || "verify_failed"} ${response.data.result.errorMessage || ""}`.trim(),
      });
    });
  }

  async function updateWizardStep(field: "scopesExported" | "eventsConfirmed" | "callbacksConfirmed" | "menusConfirmed" | "published", value: boolean) {
    if (!activeApp) {
      return;
    }
    await runAction(`wizard-${field}`, async () => {
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(activeApp.id)}/wizard`, "PATCH", {
        [field]: value,
      });
      await loadData(activeApp.id);
      setNotice({ tone: value ? "good" : "warn", message: value ? "已更新向导进度。" : "已撤销对应的向导确认状态。" });
    });
  }

  async function copyScopesJSON() {
    const app = activeApp;
    if (!app) {
      return;
    }
    await runAction("copy-scopes", async () => {
      if (navigator.clipboard?.writeText) {
        await navigator.clipboard.writeText(scopesJSON);
      }
      await sendJSON<FeishuAppResponse>(`/api/setup/feishu/apps/${encodeURIComponent(app.id)}/wizard`, "PATCH", {
        scopesExported: true,
      });
      await loadData(app.id);
      setNotice({ tone: "good", message: "Scopes JSON 已复制，并标记为已导出。" });
    });
  }

  async function applyVSCode(mode: string) {
    if (!vscode) {
      return;
    }
    await runAction(`vscode-${mode}`, async () => {
      const response = await sendJSON<VSCodeDetectResponse>("/api/setup/vscode/apply", "POST", { mode });
      setVSCode(response);
      setVSCodeError("");
      setNotice({ tone: "good", message: `VS Code 集成已切换到 ${mode}。` });
    });
  }

  async function reinstallShim() {
    if (!vscode) {
      return;
    }
    await runAction("reinstall-shim", async () => {
      const response = await sendJSON<VSCodeDetectResponse>("/api/setup/vscode/reinstall-shim", "POST");
      setVSCode(response);
      setVSCodeError("");
      setNotice({ tone: "good", message: "已重新安装 managed shim。" });
    });
  }

  async function finishSetup() {
    await runAction("finish-setup", async () => {
      const response = await sendJSON<SetupCompleteResponse>("/api/setup/complete", "POST");
      setFinishInfo(response);
      setNotice(null);
    });
  }

  if (finishInfo && bootstrap) {
    return (
      <ShellFrame
        routeLabel="Setup Completed"
        title="安装向导已完成"
        subtitle="Setup access 已经关闭。当前页面保留最终结果摘要，避免远程 session 结束后立即失去上下文。"
        nav={[
          { label: "完成摘要", href: "#summary" },
          { label: "后续动作", href: "#next" },
        ]}
      >
        <Panel id="summary" title="完成摘要" description="这一阶段会主动关闭 setup access，并清除 setup session cookie。">
          <StatGrid>
            <StatCard label="Phase" value={bootstrap.phase} tone="accent" detail="setup flow closed" />
            <StatCard label="已验证 App" value={verifiedApps.length} detail={`${apps.length} total apps`} />
            <StatCard label="VS Code 模式" value={vscode?.currentMode || "unknown"} detail={vscodeReadinessText(vscode)} />
            <StatCard label="Admin URL" value="ready" detail={finishInfo.adminURL} />
          </StatGrid>
          <div className="notice-banner good">{finishInfo.message}</div>
        </Panel>

        <Panel id="next" title="后续动作" description="远程 SSH setup 在这一阶段不会自动升级成远程 admin session。">
          <DataList
            items={[
              {
                title: bootstrap.session.trustedLoopback ? "打开本地管理页" : "管理页访问限制",
                meta: bootstrap.session.trustedLoopback ? finishInfo.adminURL : "localhost only",
                detail: bootstrap.session.trustedLoopback
                  ? "当前请求来自 loopback，后续可以直接继续本地管理页。"
                  : "远程 setup access 已关闭；正式 admin 访问当前仍限定在 localhost。",
                tone: bootstrap.session.trustedLoopback ? "good" : "warn",
              },
              {
                title: "飞书长连接",
                meta: verifiedApps.length > 0 ? "verified" : "pending",
                detail: verifiedApps.length > 0 ? "至少一个飞书 App 已通过验证，可继续回到飞书平台完成最终人工确认。" : "如果还没有完成验证，建议重新进入 setup 或在本地 admin 中继续调整。",
                tone: verifiedApps.length > 0 ? "good" : "warn",
              },
            ]}
          />
        </Panel>
      </ShellFrame>
    );
  }

  return (
    <ShellFrame
      routeLabel="Setup Session"
      title="安装与接入向导"
      subtitle="多飞书 App、平台 checklist、长连接验证和 VS Code 集成现在都可以在这一页里完成。"
      nav={[
        { label: "当前状态", href: "#overview" },
        { label: "飞书 App", href: "#apps" },
        { label: "平台 Checklist", href: "#checklist" },
        { label: "VS Code", href: "#vscode" },
        { label: "完成 Setup", href: "#finish" },
      ]}
      actions={
        <button className="secondary-button" type="button" onClick={() => void loadData(activeApp?.id)} disabled={busyAction !== ""}>
          刷新状态
        </button>
      }
    >
      {!bootstrap && !error ? <LoadingState title="正在初始化 Setup 页面" description="读取 bootstrap、飞书 App、manifest 和 VS Code 检测结果。" /> : null}
      {error ? <ErrorState title="无法加载 Setup 状态" description="setup shell 已就位，但当前状态读取失败。" detail={error} /> : null}
      {bootstrap && manifest ? (
        <>
          <Panel id="overview" title="当前状态" description="Setup session 现在支持跨多个步骤持续完成，不会在首个 App 保存后直接失效。">
            <StatGrid>
              <StatCard label="Phase" value={bootstrap.phase} tone={bootstrap.setupRequired ? "warn" : "accent"} detail={bootstrap.setupRequired ? "仍需完成 setup" : "setup ready to finish"} />
              <StatCard label="飞书 App" value={apps.length} detail={`已验证 ${verifiedApps.length} / runtime ${bootstrap.feishu.runtimeConfiguredApps}`} />
              <StatCard label="VS Code" value={vscode?.currentMode || "unavailable"} detail={vscodeReadinessText(vscode)} />
              <StatCard label="会话范围" value={bootstrap.session.scope || "unknown"} detail={bootstrap.session.trustedLoopback ? "loopback trusted" : "setup session cookie"} />
            </StatGrid>
            <DefinitionList
              items={[
                { label: "Config Path", value: bootstrap.config.path },
                { label: "Admin URL", value: bootstrap.admin.url },
                { label: "Setup URL", value: bootstrap.admin.setupURL || "not exposed" },
                { label: "SSH Session", value: bootstrap.sshSession ? "yes" : "no" },
                { label: "Relay Server URL", value: bootstrap.relay.serverURL },
                { label: "Session Expires", value: bootstrap.session.expiresAt ? formatDateTime(bootstrap.session.expiresAt) : "n/a" },
              ]}
            />
            <GatewayPanel gateways={bootstrap.gateways ?? []} />
            {notice ? <div className={`notice-banner ${notice.tone}`}>{notice.message}</div> : null}
            {!bootstrap.setupRequired ? (
              <div className="notice-banner good">飞书凭证已经具备 runtime 条件；你现在可以继续完成人工 checklist、VS Code 集成，然后主动关闭 setup access。</div>
            ) : null}
          </Panel>

          <Panel
            id="apps"
            title="飞书 App 向导"
            description="V1 就按多 App 同时在线设计。先选一个 App，再在右侧完成凭证、验证和平台 checklist。"
            actions={
              <button className="secondary-button" type="button" onClick={beginNewApp} disabled={busyAction !== ""}>
                新建 App
              </button>
            }
          >
            <div className="setup-two-column">
              <div className="app-list-grid">
                {apps.map((app) => (
                  <button key={app.id} type="button" className={`app-card${selectedID === app.id ? " selected" : ""}`} onClick={() => selectApp(app)}>
                    <div className="app-card-head">
                      <strong>{app.name || app.id}</strong>
                      <StatusBadge value={app.status?.state || (app.enabled ? "configured" : "disabled")} tone={statusTone(app.status?.state)} />
                    </div>
                    <p>{app.id}</p>
                    <div className="app-card-flags">
                      <StatusBadge value={app.hasSecret ? "secret ready" : "secret missing"} tone={app.hasSecret ? "good" : "warn"} />
                      <StatusBadge value={app.wizard?.connectionVerifiedAt ? "verified" : "unverified"} tone={app.wizard?.connectionVerifiedAt ? "good" : "warn"} />
                    </div>
                  </button>
                ))}
                <button type="button" className={`app-card app-card-create${selectedID === newAppID ? " selected" : ""}`} onClick={beginNewApp}>
                  <strong>创建新 App</strong>
                  <p>为未来多机器人同时在线预留新的配置项。</p>
                </button>
              </div>

              <div className="wizard-editor">
                <div className="form-grid">
                  <label className="field">
                    <span>Gateway ID</span>
                    <input value={draft.id} placeholder="main-bot" disabled={!draft.isNew} onChange={(event) => setDraft((current) => ({ ...current, id: event.target.value }))} />
                  </label>
                  <label className="field">
                    <span>显示名称</span>
                    <input value={draft.name} placeholder="Main Bot" onChange={(event) => setDraft((current) => ({ ...current, name: event.target.value }))} />
                  </label>
                  <label className="field">
                    <span>App ID</span>
                    <input value={draft.appId} placeholder="cli_xxx" onChange={(event) => setDraft((current) => ({ ...current, appId: event.target.value }))} />
                  </label>
                  <label className="field">
                    <span>App Secret</span>
                    <input
                      type="password"
                      value={draft.appSecret}
                      placeholder={activeApp?.hasSecret ? "留空表示保留现有 secret" : "secret_xxx"}
                      onChange={(event) => setDraft((current) => ({ ...current, appSecret: event.target.value }))}
                    />
                  </label>
                </div>
                <label className="checkbox-row">
                  <input type="checkbox" checked={draft.enabled} onChange={(event) => setDraft((current) => ({ ...current, enabled: event.target.checked }))} />
                  <span>保存后立即启用这个飞书 App</span>
                </label>

                <div className="button-row">
                  <button className="primary-button" type="button" onClick={() => void saveApp()} disabled={busyAction !== ""}>
                    {draft.isNew ? "保存并创建 App" : "保存 App 配置"}
                  </button>
                  <button className="secondary-button" type="button" onClick={() => void verifyApp()} disabled={!activeApp || busyAction !== ""}>
                    验证长连接
                  </button>
                  <button className="danger-button" type="button" onClick={() => void deleteApp()} disabled={!activeApp || activeApp.readOnly || busyAction !== ""}>
                    删除 App
                  </button>
                </div>

                {activeApp ? (
                  <>
                    <div className="wizard-progress">
                      {wizardChecklistItems.map((item) => (
                        <div key={item.label} className="wizard-step">
                          <StatusBadge value={item.done ? "done" : "pending"} tone={item.done ? "good" : "warn"} />
                          <div>
                            <strong>{item.label}</strong>
                            <p>{item.timestamp ? formatDateTime(item.timestamp) : "尚未记录"}</p>
                          </div>
                        </div>
                      ))}
                    </div>
                    {activeApp.readOnly ? <div className="notice-banner warn">当前 App 由运行时环境变量接管，setup 页面只能查看状态，不能修改配置。</div> : null}
                  </>
                ) : (
                  <div className="inline-note">
                    <StatusBadge value="Draft" tone="neutral" />
                    <span>先填写并保存一个飞书 App，后面的验证和 checklist 才会写入真实配置。</span>
                  </div>
                )}
              </div>
            </div>
          </Panel>

          <Panel id="checklist" title="飞书平台 Checklist" description="manifest 和 scopes JSON 都来自后端 source of truth；手工项会回写到每个 App 的 wizard 状态。">
            <div className="checklist-grid">
              <div className="checklist-column">
                <h4>Scopes Import JSON</h4>
                <textarea className="code-textarea" readOnly value={scopesJSON} />
                <div className="button-row">
                  <button className="secondary-button" type="button" onClick={() => void copyScopesJSON()} disabled={!activeApp || busyAction !== ""}>
                    复制并标记已导出
                  </button>
                  <button className="ghost-button" type="button" onClick={() => void updateWizardStep("scopesExported", false)} disabled={!activeApp || busyAction !== ""}>
                    撤销导出标记
                  </button>
                </div>
                <div className="manifest-block">
                  <h4>事件订阅</h4>
                  <ul className="token-list">
                    {manifest.events.map((item) => (
                      <li key={item.event}>
                        <code>{item.event}</code>
                        <span>{item.purpose || "需要手工订阅"}</span>
                      </li>
                    ))}
                  </ul>
                </div>
                <div className="manifest-block">
                  <h4>机器人菜单</h4>
                  <ul className="token-list">
                    {manifest.menus.map((item) => (
                      <li key={item.key}>
                        <code>{item.key}</code>
                        <span>{item.name}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>

              <div className="checklist-column">
                <h4>手工确认</h4>
                <div className="checkbox-card-list">
                  <label className="checkbox-card">
                    <input type="checkbox" checked={Boolean(activeApp?.wizard?.eventsConfirmedAt)} disabled={!activeApp || busyAction !== ""} onChange={(event) => void updateWizardStep("eventsConfirmed", event.target.checked)} />
                    <div>
                      <strong>事件订阅已完成</strong>
                      <p>我已经在飞书平台里把 manifest 里的事件都订阅好了。</p>
                    </div>
                  </label>
                  <label className="checkbox-card">
                    <input type="checkbox" checked={Boolean(activeApp?.wizard?.callbacksConfirmedAt)} disabled={!activeApp || busyAction !== ""} onChange={(event) => void updateWizardStep("callbacksConfirmed", event.target.checked)} />
                    <div>
                      <strong>卡片 / 回调已配置</strong>
                      <p>我已经完成了卡片 action 和相关回调入口配置。</p>
                    </div>
                  </label>
                  <label className="checkbox-card">
                    <input type="checkbox" checked={Boolean(activeApp?.wizard?.menusConfirmedAt)} disabled={!activeApp || busyAction !== ""} onChange={(event) => void updateWizardStep("menusConfirmed", event.target.checked)} />
                    <div>
                      <strong>机器人菜单已创建</strong>
                      <p>manifest 里的菜单 key 已经全部加到机器人配置里。</p>
                    </div>
                  </label>
                  <label className="checkbox-card">
                    <input type="checkbox" checked={Boolean(activeApp?.wizard?.publishedAt)} disabled={!activeApp || busyAction !== ""} onChange={(event) => void updateWizardStep("published", event.target.checked)} />
                    <div>
                      <strong>机器人版本已发布</strong>
                      <p>飞书平台的配置变更已经发版，允许真实消息流开始工作。</p>
                    </div>
                  </label>
                </div>
                <div className="manifest-block">
                  <h4>推荐检查顺序</h4>
                  <ul className="ordered-checklist">
                    {manifest.checklist.map((section) => (
                      <li key={section.area}>
                        <strong>{section.area}</strong>
                        <span>{section.items.join(" ")}</span>
                      </li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          </Panel>

          <Panel id="vscode" title="VS Code 集成" description="SSH 默认推荐 managed shim；本机默认推荐 editor settings。这里直接复用后端检测结果，不依赖 `code` CLI。">
            {vscodeError ? <div className="notice-banner warn">VS Code 检测暂时不可用：{vscodeError}</div> : null}
            <StatGrid>
              <StatCard label="Recommended" value={vscode?.recommendedMode || "unavailable"} tone="accent" detail={vscode?.sshSession ? "ssh session" : "local session"} />
              <StatCard label="Current Mode" value={vscode?.currentMode || "unknown"} detail={vscode?.currentBinary || "unavailable"} />
              <StatCard label="Settings" value={vscode?.settings.matchesBinary ? "ready" : "pending"} detail={vscode?.settings.path || "unavailable"} />
              <StatCard label="Managed Shim" value={vscode?.latestShim.matchesBinary ? "ready" : "pending"} detail={vscode?.latestBundleEntrypoint || "bundle not detected"} />
            </StatGrid>
            <div className="button-row">
              <button className="primary-button" type="button" onClick={() => void applyVSCode(vscode?.recommendedMode || "editor_settings")} disabled={!vscode || busyAction !== ""}>
                应用推荐模式
              </button>
              <button className="secondary-button" type="button" onClick={() => void applyVSCode("editor_settings")} disabled={!vscode || busyAction !== ""}>
                写入 settings.json
              </button>
              <button className="secondary-button" type="button" onClick={() => void applyVSCode("managed_shim")} disabled={!vscode || busyAction !== ""}>
                安装 managed shim
              </button>
              <button className="ghost-button" type="button" onClick={() => void reinstallShim()} disabled={!vscode?.needsShimReinstall || busyAction !== ""}>
                重新安装 shim
              </button>
            </div>
            <DefinitionList
              items={[
                { label: "Current Binary", value: vscode?.currentBinary || "unavailable" },
                { label: "Install State Path", value: vscode?.installStatePath || "unavailable" },
                { label: "Latest Bundle", value: vscode?.latestBundleEntrypoint || "not detected" },
                { label: "Recorded Bundle", value: vscode?.recordedBundleEntrypoint || "not recorded" },
                { label: "Needs Reinstall", value: vscode?.needsShimReinstall ? "yes" : "no" },
                { label: "Settings Target", value: vscode?.settings.path || "unavailable" },
              ]}
            />
          </Panel>

          <Panel id="finish" title="完成 Setup" description="这一步会关闭 setup access，并清除 setup session cookie。之后远程 SSH 页面不会自动升级成远程 admin session。">
            <DataList
              items={[
                {
                  title: "凭证条件",
                  meta: bootstrap.setupRequired ? "not ready" : "ready",
                  detail: bootstrap.setupRequired ? "还没有可供 runtime 生效的飞书 App。" : "已经有可供 runtime 生效的飞书 App。",
                  tone: bootstrap.setupRequired ? "warn" : "good",
                },
                {
                  title: "验证条件",
                  meta: `${verifiedApps.length} verified`,
                  detail: verifiedApps.length > 0 ? "至少一个飞书 App 已通过真实连通性验证。" : "建议至少完成一个 App 的验证后再结束 setup。",
                  tone: verifiedApps.length > 0 ? "good" : "warn",
                },
                {
                  title: "VS Code 状态",
                  meta: vscode?.currentMode || "unavailable",
                  detail: vscodeReadinessText(vscode),
                  tone: vscodeIsReady(vscode) ? "good" : "warn",
                },
              ]}
            />
            <div className="button-row">
              <button className="primary-button" type="button" onClick={() => void finishSetup()} disabled={busyAction !== "" || finishDisabledReason !== ""}>
                完成 Setup 并关闭远程入口
              </button>
              {finishDisabledReason ? <p className="form-hint">{finishDisabledReason}</p> : null}
            </div>
          </Panel>
        </>
      ) : null}
    </ShellFrame>
  );
}

function appToDraft(app: FeishuAppSummary): SetupDraft {
  return {
    isNew: false,
    id: app.id,
    name: app.name || "",
    appId: app.appId || "",
    appSecret: "",
    enabled: app.enabled,
  };
}

function syncDraftSelection(
  apps: FeishuAppSummary[],
  preferredID: string,
  setSelectedID: (value: string) => void,
  setDraft: (value: SetupDraft) => void,
) {
  const preferredApp = apps.find((app) => app.id === preferredID);
  if (preferredApp) {
    setSelectedID(preferredApp.id);
    setDraft(appToDraft(preferredApp));
    return;
  }
  if (apps.length > 0) {
    setSelectedID(apps[0].id);
    setDraft(appToDraft(apps[0]));
    return;
  }
  setSelectedID(newAppID);
  setDraft(emptyDraft());
}

function blankToUndefined(value: string): string | undefined {
  const trimmed = value.trim();
  return trimmed ? trimmed : undefined;
}

async function loadVSCodeState(path: string): Promise<{ data: VSCodeDetectResponse | null; error: string }> {
  try {
    return {
      data: await requestJSON<VSCodeDetectResponse>(path),
      error: "",
    };
  } catch (err: unknown) {
    return {
      data: null,
      error: formatError(err),
    };
  }
}

function statusTone(state?: string): "neutral" | "good" | "warn" | "danger" {
  switch (state) {
    case "connected":
      return "good";
    case "connecting":
    case "degraded":
      return "warn";
    case "auth_failed":
      return "danger";
    default:
      return "neutral";
  }
}

function formatDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function vscodeIsReady(vscode: VSCodeDetectResponse | null): boolean {
  if (!vscode) {
    return false;
  }
  if (vscode.recommendedMode === "managed_shim") {
    return vscode.latestShim.matchesBinary;
  }
  return vscode.settings.matchesBinary;
}

function vscodeReadinessText(vscode: VSCodeDetectResponse | null): string {
  if (!vscode) {
    return "尚未检测";
  }
  if (vscodeIsReady(vscode)) {
    return "当前推荐模式已就绪。";
  }
  if (vscode.recommendedMode === "managed_shim" && !vscode.latestBundleEntrypoint) {
    return "还没有检测到可替换的 VS Code 扩展 bundle。";
  }
  if (vscode.needsShimReinstall) {
    return "检测到扩展已升级，建议重新安装 shim。";
  }
  return "当前模式还没有指向最新的 wrapper binary。";
}

function GatewayPanel(props: { gateways: GatewayStatus[] }) {
  if (!props.gateways.length) {
    return (
      <div className="inline-note">
        <StatusBadge value="No Runtime Gateways" tone="neutral" />
        <span>当前还没有运行中的飞书长连接，这与 setup 阶段预期一致。</span>
      </div>
    );
  }
  return (
    <div className="gateway-list">
      {props.gateways.map((gateway) => (
        <div key={gateway.gatewayId} className="gateway-card">
          <div className="gateway-head">
            <strong>{gateway.name || gateway.gatewayId}</strong>
            <StatusBadge value={gateway.state} tone={statusTone(gateway.state)} />
          </div>
          <p>{gateway.lastError || "当前没有额外错误。完成平台 checklist 后可以继续观察连接状态。"}</p>
        </div>
      ))}
    </div>
  );
}
