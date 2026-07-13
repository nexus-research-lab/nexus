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
  mode?: "default" | "acceptEdits" | "plan" | "bypassPermissions";
  directories?: string[];
  destination?: PermissionDestination;
}

export interface PendingPermission {
  request_id: string;
  tool_name: string;
  tool_input: ToolInput;
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
}

export interface PermissionDecisionPayload {
  request_id: string;
  decision: PermissionDecision;
  user_answers?: UserQuestionAnswer[];
  updated_permissions?: PermissionUpdate[];
  message?: string;
  interrupt?: boolean;
}
