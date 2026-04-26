import type { APIErrorShape, JSONResult } from "../../../lib/api";
import type {
  FeishuAppSummary,
  OnboardingWorkflowResponse,
  OnboardingWorkflowStage,
} from "../../../lib/types";
import {
  buildAdminFeishuVerifySuccessMessage,
  buildSetupFeishuVerifySuccessMessage,
} from "../helpers";
import type {
  AllowedActionCarrier,
  OnboardingSurfaceMode,
  RequirementTableRow,
} from "./types";

export const defaultQRCodePollIntervalSeconds = 5;
export const vscodeApplyTimeoutMs = 10_000;
export const vscodeDetectRecoveryTimeoutMs = 5_000;

export function buildVerifySuccessMessage(
  mode: OnboardingSurfaceMode,
  app: FeishuAppSummary,
  mutation?: { message?: string; requiresNewChat?: boolean },
  duration?: number,
): string {
  if (mode === "setup") {
    return buildSetupFeishuVerifySuccessMessage(app, mutation);
  }
  return buildAdminFeishuVerifySuccessMessage(app, duration || 0);
}

export function buildWorkflowPath(apiBasePath: string, preferredAppID: string): string {
  if (!preferredAppID.trim()) {
    return `${apiBasePath}/onboarding/workflow`;
  }
  return `${apiBasePath}/onboarding/workflow?app=${encodeURIComponent(preferredAppID)}`;
}

export function syntheticConnectionStage(): OnboardingWorkflowStage {
  return {
    id: "connect",
    title: "飞书连接",
    status: "pending",
    summary: "接入并验证一个可用的飞书应用。",
    blocking: false,
    optional: false,
    allowedActions: ["start_qr", "submit_manual"],
  };
}

export function stepTitle(step: "events" | "callback" | "menu"): string {
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

export function workflowStageByID(
  workflow: OnboardingWorkflowResponse | null,
  stageID: string,
): OnboardingWorkflowStage | undefined {
  return workflow?.stages.find((stage) => stage.id === stageID);
}

export function stageAllowsAction(stage: AllowedActionCarrier, action: string): boolean {
  if (!stage) {
    return false;
  }
  return (stage.allowedActions || []).includes(action);
}

export function isResolvedStageStatus(status: string): boolean {
  return status === "complete" || status === "deferred" || status === "not_applicable";
}

export function workflowStageLabel(
  stage: OnboardingWorkflowStage,
  currentStageID: string,
): string {
  if (stage.id === currentStageID) {
    return "当前步骤";
  }
  switch (stage.status) {
    case "complete":
      return "已完成";
    case "deferred":
      return "已记录";
    case "not_applicable":
      return "不适用";
    case "blocked":
      return "等待前置";
    default:
      return "待处理";
  }
}

export function RequirementTable({
  headers,
  rows,
}: {
  headers: string[];
  rows: RequirementTableRow[];
}) {
  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            {headers.map((header) => (
              <th key={header}>{header}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.key}>
              {row.cells.map((cell, index) => (
                <td key={`${row.key}-${index}`}>{cell}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

export function readAPIError(result: JSONResult<unknown>) {
  if (result.ok) {
    return null;
  }
  const payload = result.data as APIErrorShape;
  return payload.error || null;
}
