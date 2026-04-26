import { ConnectStagePanel } from "./connect-stage";
import type { OnboardingFlowSurfaceProps } from "./types";
import { useOnboardingFlowController } from "./use-onboarding-flow-controller";
import {
  OnboardingCurrentStagePanel,
  OnboardingStageRail,
  OnboardingWorkflowOverview,
} from "./workflow-stage-panels";

export function OnboardingFlowSurface(props: OnboardingFlowSurfaceProps) {
  const controller = useOnboardingFlowController(props);

  if (controller.loading) {
    return (
      <section className="panel">
        <div className="empty-state">
          <div className="loading-dot" />
          <span>正在读取最新状态</span>
        </div>
      </section>
    );
  }

  if (controller.loadError) {
    return (
      <section className="panel">
        <div className="empty-state error">
          <strong>当前页面暂时无法打开</strong>
          <p>{controller.loadError}</p>
          <div className="button-row">
            <button
              className="secondary-button"
              type="button"
              onClick={() => void controller.retryLoad()}
            >
              重新加载
            </button>
          </div>
        </div>
      </section>
    );
  }

  if (controller.connectOnly) {
    return (
      <section className="panel">
        {controller.notice ? (
          <div className="product-notice-slot">
            <div className={`notice-banner ${controller.notice.tone}`}>
              {controller.notice.message}
            </div>
          </div>
        ) : null}
        <ConnectStagePanel controller={controller} />
      </section>
    );
  }

  return (
    <>
      {controller.notice ? (
        <div className="product-notice-slot">
          <div className={`notice-banner ${controller.notice.tone}`}>
            {controller.notice.message}
          </div>
        </div>
      ) : null}
      <div className="setup-grid">
        <OnboardingStageRail controller={controller} />
        <section className="panel step-stage">
          <OnboardingWorkflowOverview controller={controller} />
          {controller.stageID === "connect" ? (
            <ConnectStagePanel controller={controller} />
          ) : (
            <OnboardingCurrentStagePanel controller={controller} />
          )}
        </section>
      </div>
    </>
  );
}
