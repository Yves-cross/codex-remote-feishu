import { useEffect, useMemo, useState } from "react";
import { formatError, requestJSON } from "../lib/api";
import type { BootstrapState, GatewayStatus, RuntimeStatus } from "../lib/types";
import { DataList, DefinitionList, ErrorState, LoadingState, Panel, ShellFrame, StatCard, StatGrid, StatusBadge } from "../components/ui";

export function AdminRoute() {
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);
  const [runtime, setRuntime] = useState<RuntimeStatus | null>(null);
  const [error, setError] = useState<string>("");
  const [refreshTick, setRefreshTick] = useState(0);

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      try {
        const [bootstrapState, runtimeState] = await Promise.all([
          requestJSON<BootstrapState>("/api/admin/bootstrap-state"),
          requestJSON<RuntimeStatus>("/api/admin/runtime-status"),
        ]);
        if (!cancelled) {
          setBootstrap(bootstrapState);
          setRuntime(runtimeState);
          setError("");
        }
      } catch (err: unknown) {
        if (!cancelled) {
          setError(formatError(err));
        }
      }
    };

    void load();
    const interval = window.setInterval(() => {
      void load();
    }, 8000);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
    };
  }, [refreshTick]);

  const gatewayRows = useMemo(() => {
    const source = runtime?.gateways?.length ? runtime.gateways : bootstrap?.gateways ?? [];
    return source.map((gateway) => ({
      title: gateway.name || gateway.gatewayId,
      meta: gateway.lastConnectedAt ? `last connected ${formatDateTime(gateway.lastConnectedAt)}` : gateway.gatewayId,
      detail: gateway.lastError || "运行中无额外错误。",
      tone: gatewayTone(gateway.state),
    }));
  }, [bootstrap?.gateways, runtime?.gateways]);

  return (
    <ShellFrame
      routeLabel="Local Admin"
      title="本地管理控制台"
      subtitle="这一阶段先把管理页外壳、运行态概览和轮询刷新接好，后续面板会逐步替换成可操作的实例、飞书、存储和 VS Code 管理模块。"
      nav={[
        { label: "运行总览", href: "#overview" },
        { label: "网络与会话", href: "#network" },
        { label: "Gateway 健康", href: "#gateways" },
        { label: "后续模块", href: "#next" },
      ]}
      actions={
        <button className="secondary-button" type="button" onClick={() => setRefreshTick((value) => value + 1)}>
          立即刷新
        </button>
      }
    >
      {!bootstrap && !runtime && !error ? <LoadingState title="正在加载管理页概览" description="读取 bootstrap 和 runtime status，确认 embed 后的 SPA 已经能与 admin API 正常通信。" /> : null}
      {error ? <ErrorState title="无法加载管理页状态" description="当前 shell 已经接入，但页面数据读取失败。" detail={error} /> : null}
      {bootstrap && runtime ? (
        <>
          <Panel id="overview" title="运行总览" description="这些卡片用于确认当前 daemon、surface 和 remote turn 是否处于预期状态。">
            <StatGrid>
              <StatCard label="Phase" value={bootstrap.phase} tone={bootstrap.phase === "ready" ? "accent" : "warn"} detail={bootstrap.setupRequired ? "setup 未完成" : "setup 已完成"} />
              <StatCard label="Instances" value={runtime.instances.length} detail={`${runtime.surfaces.length} surfaces`} />
              <StatCard label="Remote Queue" value={runtime.pendingRemoteTurns.length} detail={`${runtime.activeRemoteTurns.length} active turns`} />
              <StatCard label="Gateways" value={(runtime.gateways ?? bootstrap.gateways ?? []).length} detail={`${bootstrap.feishu.runtimeConfiguredApps} runtime configured apps`} />
            </StatGrid>
          </Panel>

          <Panel id="network" title="网络与会话" description="当前阶段保留后端原始状态，便于确认 bind 策略和 cookie/session 行为。">
            <DefinitionList
              items={[
                { label: "Config Path", value: bootstrap.config.path },
                { label: "Admin URL", value: bootstrap.admin.url },
                { label: "Admin Listen", value: `${bootstrap.admin.listenHost}:${bootstrap.admin.listenPort}` },
                { label: "Relay Listen", value: `${bootstrap.relay.listenHost}:${bootstrap.relay.listenPort}` },
                { label: "Session Scope", value: bootstrap.session.scope || "unknown" },
                {
                  label: "Session Access",
                  value: bootstrap.session.trustedLoopback ? (
                    <StatusBadge value="trusted loopback" tone="good" />
                  ) : (
                    <StatusBadge value="session cookie" tone="neutral" />
                  ),
                },
              ]}
            />
          </Panel>

          <Panel id="gateways" title="Gateway 健康" description="多飞书 App 同时在线之后，这里会是判断整体连接状态的第一入口。">
            {gatewayRows.length ? (
              <DataList items={gatewayRows} />
            ) : (
              <div className="inline-note">
                <StatusBadge value="No Gateways" tone="neutral" />
                <span>当前尚未检测到运行中的飞书 gateway。</span>
              </div>
            )}
          </Panel>

          <Panel id="next" title="接下来要落到页面的模块" description="这一页的布局已经稳定，下一步会把这些模块替换成真正可操作的管理面板。">
            <DataList
              items={[
                {
                  title: "飞书 App 管理",
                  meta: "next milestone",
                  detail: "显示多 App 同时在线状态，支持新增、编辑、verify、enable/disable、reconnect 与 scopes 导出。",
                  tone: "warn",
                },
                {
                  title: "VS Code 集成",
                  meta: "next milestone",
                  detail: "展示 shim 是否跟上最新扩展 bundle，并提供 apply / reinstall 操作。",
                  tone: "warn",
                },
                {
                  title: "实例与存储",
                  meta: "next milestone",
                  detail: "接入 managed headless instance、image staging 和 preview drive 管理视图。",
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
