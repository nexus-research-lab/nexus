import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

import type {
  NexusOperationEvent,
  NexusOperationSnapshot,
  OperationEvidence,
  OperationPhase,
  OperationSurface,
} from "./operation-types";
import type { OperationImageSourceKind } from "./operation-image-source";

export type StageWindowKind =
  | "finder"
  | "code_editor"
  | "file_preview"
  | "markdown_reader"
  | "word_reader"
  | "presentation"
  | "pdf_reader"
  | "spreadsheet"
  | "image_viewer"
  | "browser"
  | "terminal"
  | "tasks"
  | "library"
  | "run_manifest"
  | "handoff"
  | "evidence"
  | "summary"
  | "permission_wait";

export type StageWindowPhase =
  | "opening"
  | "focused"
  | "background"
  | "minimized"
  | "closing"
  | "closed"
  | "error";

export type StageWindowLayout =
  | "primary"
  | "secondary"
  | "inspector"
  | "terminal"
  | "compact"
  | "artifact";

export interface StageHandoffSummary {
  status_label: string;
  status_detail: string;
  resume_prompt: string;
  primary_artifact?: string;
  checkpoints: Array<{
    label: string;
    value: string;
    tone: "neutral" | "success" | "warning";
  }>;
}

export interface StageWindowPayload {
  event: NexusOperationEvent;
  snapshot: NexusOperationSnapshot | null;
  target?: string | null;
  summary?: string | null;
  preview?: unknown;
  lines?: string[];
  command?: string | null;
  query?: string | null;
  srcdoc?: string | null;
  url?: string | null;
  evidence?: OperationEvidence[];
  workspace_items?: WorkspaceActivityItem[];
  related_events?: NexusOperationEvent[];
  diff_stats?: {
    additions: number;
    deletions: number;
  } | null;
  handoff_summary?: StageHandoffSummary;
  image_source?: string | null;
  image_source_kind?: OperationImageSourceKind;
  workspace_preview?: boolean;
}

export interface StageWindowState {
  id: string;
  kind: StageWindowKind;
  title: string;
  subtitle?: string | null;
  target?: string | null;
  phase: StageWindowPhase;
  z: number;
  layout: StageWindowLayout;
  payload: StageWindowPayload;
}

export interface OperationDesktopState {
  active_window_id: string | null;
  surface: OperationSurface;
  phase: OperationPhase;
  windows: StageWindowState[];
  minimized: StageWindowState[];
  artifacts: StageWindowState[];
}
