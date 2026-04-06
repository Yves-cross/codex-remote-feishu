import { useEffect, useState } from "react";
import { formatError, requestJSON } from "../lib/api";
import type { BootstrapState, GatewayStatus } from "../lib/types";
import { DataList, DefinitionList, ErrorState, LoadingState, Panel, ShellFrame, StatCard, StatGrid, StatusBadge } from "../components/ui";

export function SetupRoute() {
  const [state, setState] = useState<BootstrapState | null>(null);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    let cancelled = false;
    requestJSON<BootstrapState>("/api/setup/bootstrap-state")
      .then((payload) => {
        if (!cancelled) {
          setState(payload);
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

  return (
    <ShellFrame
      routeLabel="Setup Session"
      title="安装与接入向导"
      subtitle="这一阶段先把 setup shell 和状态读取接通，后续步骤会在同一套布局里接入飞书凭证录入、验证和 VS Code 集成。"
      nav={[
        { label: "当前状态", href: "#overview" },
        { label: "访问信息", href: "#access" },
        { label: "向导分步", href: "#steps" },
      ]}
    >
      {!state && !error ? <LoadingState title="正在初始化 Setup 页面" description="读取 bootstrap state 以确认当前实例是否仍处于未配置状态。" /> : null}
      {error ? <ErrorState title="无法加载 Setup 状态" description="当前 setup shell 已经接入，但状态读取失败。" detail={error} /> : null}
      {state ? (
        <>
          <Panel id="overview" title="当前状态" description="这里直接显示后端 bootstrap state，确认 setup 链路和会话鉴权已经生效。">
            <StatGrid>
              <StatCard label="Phase" value={state.phase} tone={state.setupRequired ? "warn" : "accent"} detail={state.setupRequired ? "仍需完成 setup" : "setup 已完成"} />
              <StatCard label="配置文件" value={`v${state.config.version}`} detail={state.config.path} />
              <StatCard label="飞书 App" value={state.feishu.appCount} detail={`已配置 ${state.feishu.configuredAppCount} / runtime ${state.feishu.runtimeConfiguredApps}`} />
              <StatCard label="会话范围" value={state.session.scope || "unknown"} detail={state.session.trustedLoopback ? "loopback trusted session" : "token/cookie session"} />
            </StatGrid>
            <GatewayPanel gateways={state.gateways ?? []} />
          </Panel>

          <Panel id="access" title="访问与绑定信息" description="这些值决定 setup 如何暴露、后续用户应该从哪里继续完成配置。">
            <DefinitionList
              items={[
                { label: "Admin URL", value: state.admin.url },
                { label: "Setup URL", value: state.admin.setupURL || "not exposed" },
                { label: "Admin Listen", value: `${state.admin.listenHost}:${state.admin.listenPort}` },
                { label: "Relay Listen", value: `${state.relay.listenHost}:${state.relay.listenPort}` },
                { label: "Relay Server URL", value: state.relay.serverURL },
                { label: "SSH Session", value: state.sshSession ? "yes" : "no" },
              ]}
            />
          </Panel>

          <Panel id="steps" title="向导分步骨架" description="下一阶段会在这些分区里接入真正的写接口。当前先把页面结构和后端状态面连接起来。">
            <DataList
              items={[
                {
                  title: "1. 飞书 App 凭证",
                  meta: "planned next",
                  detail: "录入 App ID / App Secret，随后执行真实长连接验证，并把 wizard 进度持久化到统一配置里。",
                  tone: "warn",
                },
                {
                  title: "2. 权限、事件与回调检查单",
                  meta: "planned next",
                  detail: "展示后端 manifest source of truth，导出 scopes import JSON，并引导手工补齐事件、回调和机器人发布。",
                  tone: "warn",
                },
                {
                  title: "3. VS Code 集成",
                  meta: "planned next",
                  detail: "根据是否处于 SSH session 自动推荐 managed shim 或 editor settings，并给出一键应用。",
                  tone: "warn",
                },
              ]}
            />
          </Panel>
        </>
      ) : null}
    </ShellFrame>
  );
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
            <StatusBadge value={gateway.state} tone={gatewayTone(gateway.state)} />
          </div>
          <p>{gateway.lastError || "尚无错误，等待下一阶段接入凭证与验证流程。"}</p>
        </div>
      ))}
    </div>
  );
}

function gatewayTone(state: string): "neutral" | "good" | "warn" | "danger" {
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
