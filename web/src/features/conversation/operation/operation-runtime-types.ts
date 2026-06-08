import type { WorkspaceDiffStats } from "@/types/app/workspace-live";
import type {
  PermissionDecision,
  PermissionInteractionMode,
  PermissionRiskLevel,
} from "@/types/conversation/permission";

import type { OperationPhase } from "./operation-types";

export type OperationRuntimeEventType =
  | "tool_start"
  | "tool_delta"
  | "tool_end"
  | "artifact_update"
  | "permission_request"
  | "permission_resolved"
  | "round_handoff";

export interface OperationRuntimeArtifact {
  kind: "workspace_file" | "html" | "handoff" | "unknown";
  path?: string | null;
  status?: string | null;
  live_content?: string | null;
  diff_stats?: WorkspaceDiffStats | null;
  preview?: unknown;
}

export interface OperationRuntimeEvent {
  id: string;
  event_type: OperationRuntimeEventType;
  session_key: string | null;
  round_id: string;
  agent_id: string;
  message_id?: string | null;
  tool_use_id?: string | null;
  tool_name?: string | null;
  phase: OperationPhase;
  timestamp: number;
  input?: Record<string, unknown> | null;
  delta?: unknown;
  result?: unknown;
  artifact?: OperationRuntimeArtifact | null;
  permission_request_id?: string | null;
  permission_decision?: PermissionDecision | null;
  permission_interaction_mode?: PermissionInteractionMode | null;
  permission_risk_level?: PermissionRiskLevel | null;
  source_event_id?: string | null;
}
