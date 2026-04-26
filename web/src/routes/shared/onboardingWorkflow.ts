import type {
  OnboardingWorkflowAppStep,
  OnboardingWorkflowMachineStep,
  OnboardingWorkflowPermission,
  OnboardingWorkflowResponse,
  OnboardingWorkflowStage,
} from "../../lib/types";

export function buildOnboardingWorkflowPath(apiBase: string, preferredAppID: string): string {
  if (!preferredAppID.trim()) {
    return `${apiBase}/onboarding/workflow`;
  }
  return `${apiBase}/onboarding/workflow?app=${encodeURIComponent(preferredAppID)}`;
}

export function workflowStageByID(
  workflow: OnboardingWorkflowResponse | null,
  id: string,
): OnboardingWorkflowStage | null {
  if (!workflow) {
    return null;
  }
  return workflow.stages.find((stage) => stage.id === id) || null;
}

export function stageAllowsAction(
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

export function isResolvedStageStatus(status: string): boolean {
  return status === "complete" || status === "deferred" || status === "not_applicable";
}

export function workflowStageLabel(stage: OnboardingWorkflowStage, currentStageID: string): string {
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

export function onboardingStepTitle(step: "events" | "callback" | "menu"): string {
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
