import type { UserQuestionAnswer } from "./ask-user-question";
import type { ToolInput } from "../../system/sdk";

export type PermissionRiskLevel = "low" | "medium" | "high";
export type PermissionDecision = "allow" | "deny";
export type PermissionInteractionMode = "permission" | "question";
export type PermissionUpdateType =
  | "addRules"
  | "replaceRules"
  | "removeRules"
  | "setMode"
  | "addDirectories"
  | "removeDirectories";
export type PermissionBehavior = "allow" | "deny" | "ask";
export type PermissionDestination =
  | "userSettings"
  | "projectSettings"
  | "localSettings"
  | "session";

export interface PermissionRule {
  tool_name: string;
  rule_content?: string | null;
}

export interface PermissionUpdate {
  type: PermissionUpdateType;
  rules?: PermissionRule[];
  behavior?: PermissionBehavior;
  mode?: "default" | "acceptEdits" | "plan" | "dontAsk" | "bypassPermissions";
  directories?: string[];
  destination?: PermissionDestination;
}

export interface ConfigurationSecretSlot {
  id: string;
  path: string;
}

export interface AutomationPermissionContext {
  allow_task: boolean;
  job_id: string;
  kind: string;
  policy_revision: number;
  run_id?: string;
  task_name: string;
}

export type ConfigurationSecrets = Record<string, string>;

export interface PendingPermission {
  request_id: string;
  tool_name: string;
  tool_input: ToolInput;
  configuration_secret_slots?: ConfigurationSecretSlot[];
  session_key?: string | null;
  agent_id?: string | null;
  message_id?: string | null;
  round_id?: string | null;
  agent_round_id?: string | null;
  tool_use_id?: string | null;
  interaction_mode?: PermissionInteractionMode;
  risk_level?: PermissionRiskLevel;
  risk_label?: string;
  summary?: string;
  suggestions?: PermissionUpdate[];
  expires_at?: string;
  source?: "automation";
  automation?: AutomationPermissionContext;
}

export interface PermissionDecisionPayload {
  request_id: string;
  decision: PermissionDecision;
  configuration_secrets?: ConfigurationSecrets;
  user_answers?: UserQuestionAnswer[];
  updated_permissions?: PermissionUpdate[];
  automation_scope?: "once" | "task";
  message?: string;
  interrupt?: boolean;
}
