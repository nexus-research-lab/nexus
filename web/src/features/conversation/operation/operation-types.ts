import type { WorkspaceActivityItem } from "@/types/app/workspace-live";
import type {
  PermissionDecision,
  PermissionInteractionMode,
} from "@/types/conversation/permission";

import type { OperationRuntimeEvent } from "./operation-runtime-types";

export type OperationKind =
  | "workspace_inspect"
  | "workspace_read"
  | "workspace_search"
  | "workspace_edit"
  | "command_run"
  | "command_stop"
  | "web_research"
  | "context_read"
  | "task_delegate"
  | "task_progress"
  | "plan_update"
  | "human_gate"
  | "artifact_update"
  | "round_summary"
  | "unknown";

export type OperationSurface =
  | "workspace"
  | "editor"
  | "terminal"
  | "web"
  | "knowledge"
  | "task"
  | "conversation"
  | "summary"
  | "fallback";

export type OperationPhase =
  | "queued"
  | "running"
  | "waiting"
  | "done"
  | "error"
  | "cancelled";

export interface OperationEvidence {
  type:
    | "file"
    | "diff"
    | "terminal"
    | "url"
    | "skill"
    | "task"
    | "permission"
    | "artifact"
    | "error"
    | "status";
  label: string;
  value?: string | null;
  preview?: unknown;
}

export interface NexusOperationEvent {
  id: string;
  session_key: string;
  round_id: string;
  agent_id: string;
  message_id?: string | null;
  tool_use_id?: string | null;
  tool_name?: string | null;
  kind: OperationKind;
  surface: OperationSurface;
  phase: OperationPhase;
  title: string;
  target?: string | null;
  summary?: string | null;
  input_preview?: Record<string, unknown> | null;
  result_preview?: unknown;
  evidence?: OperationEvidence[];
  permission_request_id?: string | null;
  permission_decision?: PermissionDecision | null;
  permission_interaction_mode?: PermissionInteractionMode | null;
  started_at?: number;
  updated_at: number;
  ended_at?: number | null;
}

export interface NexusOperationSnapshot {
  key: string;
  session_key: string | null;
  active_event: NexusOperationEvent | null;
  events: NexusOperationEvent[];
  runtime_events: OperationRuntimeEvent[];
  recent_evidence: OperationEvidence[];
  workspace_events: WorkspaceActivityItem[];
  updated_at: number;
}
