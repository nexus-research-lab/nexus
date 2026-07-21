import type { StageWindowKind } from "../operation-desktop-types";

const DESKTOP_WINDOW_KINDS = new Set<StageWindowKind>([
  "browser",
  "code_editor",
  "file_preview",
  "finder",
  "handoff",
  "image_viewer",
  "markdown_reader",
  "pdf_reader",
  "permission_wait",
  "presentation",
  "run_manifest",
  "spreadsheet",
  "tasks",
  "terminal",
  "word_reader",
]);

const FLUSH_CONTENT_WINDOW_KINDS = new Set<StageWindowKind>([
  "browser",
  "code_editor",
  "file_preview",
  "finder",
  "handoff",
  "image_viewer",
  "markdown_reader",
  "pdf_reader",
  "presentation",
  "run_manifest",
  "spreadsheet",
  "tasks",
  "terminal",
  "word_reader",
]);

export function isStageDesktopWindowKind(kind: StageWindowKind): boolean {
  return DESKTOP_WINDOW_KINDS.has(kind);
}

export function windowContentModeForKind(kind: StageWindowKind): "flush" | "inset" {
  return FLUSH_CONTENT_WINDOW_KINDS.has(kind) ? "flush" : "inset";
}
