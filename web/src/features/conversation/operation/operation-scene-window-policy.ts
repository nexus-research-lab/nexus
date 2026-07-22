import type {
  StageWindowKind,
  StageWindowPhase,
} from "./operation-desktop-types";
import type { NexusOperationEvent } from "./operation-types";

export function shouldOpenFinderWindow(
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
  if (isRoundReviewEvent(event)) {
    return context.workspace_item_count > 0;
  }
  return false;
}

export function shouldOpenHtmlBrowserWindow(
  event: NexusOperationEvent,
  has_html_artifact: boolean,
): boolean {
  if (!has_html_artifact) {
    return false;
  }
  if (event.surface === "terminal") {
    return event.phase === "running" || event.phase === "done";
  }
  return event.surface === "web"
    || event.surface === "summary"
    || event.kind === "round_summary";
}

export function supportingWindowPhase(
  kind: StageWindowKind,
  is_focused: boolean,
  context: {
    has_browser_artifact: boolean;
    is_review_event: boolean;
  },
): StageWindowPhase {
  if (is_focused && !context.is_review_event) {
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

export function isDesktopToolActivityEvent(event: NexusOperationEvent): boolean {
  return event.surface !== "conversation"
    && event.kind !== "round_summary";
}

export function isRoundReviewEvent(event: NexusOperationEvent): boolean {
  return event.kind === "round_summary" ||
    (event.surface === "summary" && (
      event.phase === "done" ||
      event.phase === "error" ||
      event.phase === "cancelled"
    ));
}

function is_document_window_kind(kind: StageWindowKind): boolean {
  return kind === "code_editor"
    || kind === "file_preview"
    || kind === "image_viewer"
    || kind === "markdown_reader"
    || kind === "pdf_reader"
    || kind === "spreadsheet"
    || kind === "presentation"
    || kind === "word_reader";
}
