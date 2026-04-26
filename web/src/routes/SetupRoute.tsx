import { useEffect, useState } from "react";
import { requestJSON } from "../lib/api";
import type { BootstrapState } from "../lib/types";
import { OnboardingFlowSurface } from "./shared/onboarding-flow";

export function SetupRoute() {
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [bootstrap, setBootstrap] = useState<BootstrapState | null>(null);

  const title = buildSetupPageTitle(bootstrap);

  useEffect(() => {
    document.title = title;
  }, [title]);

  useEffect(() => {
    let cancelled = false;
    void loadBootstrap().catch(() => {
      if (!cancelled) {
        setLoadError("当前页面暂时无法读取状态，请刷新后重试。");
        setLoading(false);
      }
    });
    return () => {
      cancelled = true;
    };
  }, []);

  async function loadBootstrap() {
    setLoading(true);
    setLoadError("");
    const bootstrapState = await requestJSON<BootstrapState>("/api/setup/bootstrap-state");
    setBootstrap(bootstrapState);
    setLoading(false);
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
                onClick={() => void loadBootstrap()}
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
      <OnboardingFlowSurface
        mode="setup"
        fallbackAdminURL={bootstrap?.admin.url || "/admin/"}
      />
    </div>
  );
}

function buildSetupPageTitle(bootstrap: BootstrapState | null): string {
  const name = bootstrap?.product.name?.trim() || "Codex Remote Feishu";
  const version = bootstrap?.product.version?.trim();
  return version ? `${name} ${version} 安装程序` : `${name} 安装程序`;
}
