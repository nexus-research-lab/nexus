import type {
  StageWindowKind,
  StageWindowPhase,
} from "./operation-desktop-types";
import type { NexusOperationEvent } from "./operation-types";

export function should_open_finder_window(
  event: NexusOperationEvent,
  context: {
    file_document_count: number;
    workspace_item_count: number;
  },
): boolean {
  if (event.surface === "workspace") {
    return event.kind === "workspace_inspect" ||
      event.kind === "workspace_search" ||
      context.file_document_count === 0;
  }
  if (is_round_review_event(event)) {
    return context.workspace_item_count > 0;
  }
  return false;
}

export function should_open_html_browser_window(
  event: NexusOperationEvent,
  has_html_artifact: boolean,
): boolean {
  if (!has_html_artifact) {
    return false;
  }
  return event.surface === "web"
    || event.surface === "terminal"
    || event.surface === "summary"
    || event.kind === "round_summary";
}

export function supporting_window_phase(
  kind: StageWindowKind,
  is_focused: boolean,
  context: {
    has_browser_artifact: boolean;
    is_review_event: boolean;
  },
): StageWindowPhase {
  if (is_focused) {
    return "focused";
  }
  if (!context.is_review_event) {
    return "background";
  }
  if (kind === "browser") {
    return "background";
  }
  if (!context.has_browser_artifact && is_document_window_kind(kind)) {
    return "background";
  }
  return "minimized";
}

export function is_desktop_tool_activity_event(event: NexusOperationEvent): boolean {
  return event.surface !== "conversation"
    && event.kind !== "round_summary";
}

export function is_round_review_event(event: NexusOperationEvent): boolean {
  return event.kind === "round_summary" ||
    (event.surface === "summary" && (
      event.phase === "done" ||
      event.phase === "error" ||
      event.phase === "cancelled"
    ));
}

function is_document_window_kind(kind: StageWindowKind): boolean {
  return kind === "code_editor"
    || kind === "image_viewer"
    || kind === "markdown_reader"
    || kind === "pdf_reader"
    || kind === "spreadsheet"
    || kind === "word_reader";
}
