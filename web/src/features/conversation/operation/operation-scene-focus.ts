/**
 * INPUT: The active operation event and available app capabilities for its round.
 * OUTPUT: Foreground app intent and preferred window kinds for event-to-window lookup.
 * POS: Pure Operation Stage focus policy; window construction stays in scene-planner.
 */
import type { StageWindowKind } from "./operation-desktop-types";
import type { NexusOperationEvent } from "./operation-types";

export type OperationFocusTarget =
  | "browser"
  | "document"
  | "finder"
  | "manifest"
  | "summary"
  | "task"
  | "terminal";

export interface OperationFocusContext {
  has_file: boolean;
  has_html_artifact: boolean;
  has_task: boolean;
  has_terminal: boolean;
  has_web: boolean;
  opens_browser: boolean;
  opens_preview: boolean;
}

export function resolveOperationFocusTarget(
  event: NexusOperationEvent,
  context: OperationFocusContext,
): OperationFocusTarget {
  if (event.phase === "waiting" && event.surface === "terminal" && context.has_terminal) {
    return "terminal";
  }
  if (event.phase === "waiting" || event.surface === "conversation") {
    return "summary";
  }
  if (event.kind === "round_summary" || (
    event.surface === "summary" &&
    (event.phase === "done" || event.phase === "error" || event.phase === "cancelled")
  )) {
    if (context.has_html_artifact || context.has_web) {
      return "browser";
    }
    if (context.has_file) {
      return "document";
    }
    if (context.has_terminal) {
      return "terminal";
    }
    if (context.has_task) {
      return "task";
    }
    return "manifest";
  }
  if (event.surface === "task" && context.has_task) {
    return "task";
  }
  if (context.opens_browser && (event.surface === "terminal" || event.surface === "web")) {
    return "browser";
  }
  if (context.opens_preview && event.surface === "terminal") {
    return "document";
  }
  if (event.surface === "knowledge") {
    return "document";
  }
  if (event.surface === "terminal" && event.phase === "done" && context.has_html_artifact) {
    return "browser";
  }
  if (event.surface === "terminal" && context.has_terminal) {
    return "terminal";
  }
  if (event.surface === "web" || (context.has_html_artifact && event.surface === "summary")) {
    return "browser";
  }
  if ((event.surface === "workspace" || event.surface === "editor") && context.has_file) {
    if (
      event.kind === "workspace_read" ||
      event.kind === "workspace_edit" ||
      event.kind === "artifact_update" ||
      event.surface === "editor"
    ) {
      return "document";
    }
    return "finder";
  }
  if (context.has_html_artifact) {
    return "browser";
  }
  if (context.has_file) {
    return "document";
  }
  return "summary";
}

export function preferredWindowKindsForEvent(event: NexusOperationEvent): StageWindowKind[] {
  if (event.surface === "terminal") {
    return ["terminal"];
  }
  if (event.surface === "web") {
    return ["browser"];
  }
  if (event.surface === "task") {
    return ["tasks"];
  }
  if (event.surface === "conversation") {
    return ["terminal", "finder", "browser"];
  }
  if (event.surface === "summary" || event.kind === "round_summary") {
    return ["handoff", "run_manifest", "summary"];
  }
  if (event.surface === "workspace") {
    if (
      event.kind === "workspace_read" ||
      event.kind === "workspace_edit" ||
      event.kind === "artifact_update"
    ) {
      return document_window_kinds("finder");
    }
    return ["finder", ...document_window_kinds()];
  }
  if (event.surface === "editor" || event.surface === "knowledge") {
    return document_window_kinds();
  }
  return ["summary"];
}

function document_window_kinds(trailing?: StageWindowKind): StageWindowKind[] {
  const kinds: StageWindowKind[] = [
    "code_editor",
    "file_preview",
    "markdown_reader",
    "word_reader",
    "presentation",
    "pdf_reader",
    "spreadsheet",
    "image_viewer",
  ];
  return trailing ? [...kinds, trailing] : kinds;
}
