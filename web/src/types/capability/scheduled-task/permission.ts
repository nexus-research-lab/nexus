/**
 * 定时任务持久审批请求、capability 与决策结果契约。
 */

import type { ApiScheduledTask } from "./task";
import type { ApiScheduledTaskRun } from "./run";

export type AutomationPermissionRequestKind =
  | "tool"
  | "script"
  | "connector_reauth"
  | "human_input";

export type AutomationPermissionRequestStatus =
  | "pending"
  | "approved"
  | "denied"
  | "superseded"
  | "cancelled";

export type AutomationPermissionDecision =
  | "allow_once"
  | "allow_task"
  | "deny"
  | "retry";

export interface AutomationPermissionDecisionInput {
  decision: AutomationPermissionDecision;
  job_id: string;
  run_id: string;
  policy_revision: number;
}

export interface AutomationPermissionResumeInput {
  request_id: string;
  policy_revision: number;
}

export interface AutomationPermissionCapability {
  tool_name: string;
  connector_id?: string | null;
  effect: "read" | "write" | "execute" | string;
  resource_scope?: string | null;
  input_fingerprint?: string | null;
}

export interface AutomationPermissionRequest {
  request_id: string;
  job_id: string;
  run_id?: string | null;
  policy_revision: number;
  kind: AutomationPermissionRequestKind;
  status: AutomationPermissionRequestStatus;
  decision?: AutomationPermissionDecision | null;
  capability: AutomationPermissionCapability;
  input_summary?: Record<string, unknown> | null;
  title?: string | null;
  description?: string | null;
  reason?: string | null;
  session_key?: string | null;
  round_id?: string | null;
  tool_use_id?: string | null;
  resume_safe: boolean;
  resolved_by_user_id?: string | null;
  created_at: string;
  updated_at: string;
  resolved_at?: string | null;
}

export interface ApiAutomationPermissionDecisionResult {
  request?: AutomationPermissionRequest | null;
  task: ApiScheduledTask;
  run?: ApiScheduledTaskRun | null;
  resume_started: boolean;
}

export interface AutomationPermissionDecisionResult
  extends Omit<ApiAutomationPermissionDecisionResult, "task" | "run"> {
  task: import("./task").ScheduledTaskItem;
  run?: import("./run").ScheduledTaskRunItem | null;
}
