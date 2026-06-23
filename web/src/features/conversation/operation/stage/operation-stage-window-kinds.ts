import type { StageWindowKind } from "../operation-desktop-types";

const DESKTOP_WINDOW_KINDS = new Set<StageWindowKind>([
  "browser",
  "code_editor",
  "finder",
  "handoff",
  "image_viewer",
  "markdown_reader",
  "pdf_reader",
  "permission_wait",
  "run_manifest",
  "spreadsheet",
  "task_board",
  "terminal",
  "word_reader",
]);

const FLUSH_CONTENT_WINDOW_KINDS = new Set<StageWindowKind>([
  "browser",
  "code_editor",
  "finder",
  "handoff",
  "image_viewer",
  "markdown_reader",
  "pdf_reader",
  "run_manifest",
  "spreadsheet",
  "task_board",
  "terminal",
  "word_reader",
]);

export function is_stage_desktop_window_kind(kind: StageWindowKind): boolean {
  return DESKTOP_WINDOW_KINDS.has(kind);
}

export function window_content_mode_for_kind(kind: StageWindowKind): "flush" | "inset" {
  return FLUSH_CONTENT_WINDOW_KINDS.has(kind) ? "flush" : "inset";
}
