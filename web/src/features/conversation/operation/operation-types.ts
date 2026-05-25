export type NexusOperationPhase =
  | "waiting"
  | "running"
  | "succeeded"
  | "failed"
  | "cancelled"
  | "completed";

export type OperationEvidenceType =
  | "status"
  | "file"
  | "diff"
  | "terminal"
  | "url"
  | "task"
  | "permission"
  | "error"
  | "skill";

export interface OperationEvidence {
  type: OperationEvidenceType;
  label: string;
  value?: string | null;
  preview?: unknown;
}

export interface NexusOperationEvent {
  tool_name?: string | null;
  kind?: string | null;
  surface?: string | null;
  phase: NexusOperationPhase;
  title?: string | null;
  summary?: string | null;
  target?: string | null;
  input_preview?: unknown;
  evidence?: OperationEvidence[] | null;
  updated_at?: string | number | null;
}

export interface NexusOperationSnapshot {
  recent_evidence?: OperationEvidence[] | null;
}
