/**
 * INPUT: User-selected workspace paths plus the current real operation event and planned windows.
 * OUTPUT: Stable, deduplicated Stage windows for files opened from the Agent OS Files app.
 * POS: User file-launch boundary; Files can open real previews without inventing tool events.
 */
import type { StageWindowKind, StageWindowState } from "./operation-desktop-types";
import {
  operationWorkspaceTargetsMatch,
  windowKindForFileTarget,
} from "./operation-file-documents";
import { normalizeWindowId } from "./operation-scene-planner-helpers";
import { buildOperationStageWindow } from "./operation-scene-window-builder";
import type { NexusOperationEvent, NexusOperationSnapshot } from "./operation-types";

const DOCUMENT_WINDOW_KINDS = new Set<StageWindowKind>([
  "browser",
  "code_editor",
  "image_viewer",
  "markdown_reader",
  "pdf_reader",
  "presentation",
  "spreadsheet",
  "word_reader",
]);

export function operationUserFileWindowId(roundId: string, path: string): string {
  return `${roundId}:user-file:${normalizeWindowId(path)}`;
}

export function appendOperationUserFilePath(paths: string[], path: string): string[] {
  const normalized = normalize_workspace_path(path);
  if (!normalized || paths.some((item) => operationWorkspaceTargetsMatch(item, normalized))) {
    return paths;
  }
  return [...paths, normalized];
}

export function findOperationWorkspaceWindow(
  windows: StageWindowState[],
  path: string,
): StageWindowState | null {
  return windows.find((window) => (
    DOCUMENT_WINDOW_KINDS.has(window.kind)
    && Boolean(window.payload.target)
    && operationWorkspaceTargetsMatch(window.payload.target ?? "", path)
  )) ?? null;
}

export function mergeOperationUserFileWindows({
  event,
  openedPaths,
  plannedWindows,
  snapshot,
}: {
  event: NexusOperationEvent;
  openedPaths: string[];
  plannedWindows: StageWindowState[];
  snapshot: NexusOperationSnapshot | null;
}): StageWindowState[] {
  const windows = [...plannedWindows];
  openedPaths.forEach((path, index) => {
    if (findOperationWorkspaceWindow(windows, path)) {
      return;
    }
    windows.push(build_user_file_window({ event, index, path, snapshot }));
  });
  return windows;
}

function build_user_file_window({
  event,
  index,
  path,
  snapshot,
}: {
  event: NexusOperationEvent;
  index: number;
  path: string;
  snapshot: NexusOperationSnapshot | null;
}): StageWindowState {
  const is_html = /\.html?$/i.test(path.split(/[?#]/, 1)[0] ?? "");
  const kind = is_html ? "browser" : windowKindForFileTarget(path, "code_editor");
  const workspace_item = snapshot?.workspace_events.find((item) => (
    operationWorkspaceTargetsMatch(item.path, path)
  ));
  return buildOperationStageWindow(event, snapshot, {
    id: `user-file:${normalizeWindowId(path)}`,
    session_id: operationUserFileWindowId(event.round_id, path),
    kind,
    layout: "primary",
    phase: "focused",
    title: basename(path),
    z: 50 + index,
    payload: {
      preview: workspace_item?.live_content ?? null,
      query: is_html ? basename(path) : null,
      related_events: [],
      srcdoc: is_html ? workspace_item?.live_content ?? null : null,
      summary: `打开 ${path}`,
      target: path,
      workspace_preview: !is_html,
    },
  });
}

function normalize_workspace_path(path: string): string | null {
  const normalized = path.trim().replace(/\\/g, "/").replace(/^\.\//, "");
  if (!normalized || normalized.startsWith("/") || normalized.split("/").includes("..")) {
    return null;
  }
  return normalized;
}

function basename(path: string): string {
  return path.split("/").filter(Boolean).at(-1) ?? path;
}
