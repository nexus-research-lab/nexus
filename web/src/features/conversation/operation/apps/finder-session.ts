/**
 * INPUT: Workspace inventory, live file activity, search query, and current Files scope.
 * OUTPUT: A stable Finder-like tree, selection, counters, and result-path projection.
 * POS: Pure Agent OS Files session model; it performs no workspace I/O.
 */
import type { WorkspaceActivityItem } from "@/types/app/workspace-live";
import type { WorkspaceFileEntry } from "@/types/agent/agent";

import type { NexusOperationEvent } from "../operation-types";
import {
  finderPreviewLines,
  resolveFinderSelectedItem,
} from "./finder-item-details";

export interface FinderTreeRow {
  depth: number;
  label: string;
  path: string;
  type: "folder" | "file";
}

export type FinderWorkspaceStatus = WorkspaceActivityItem["status"];
export type FinderViewScope = "changes" | "workspace";

export interface FinderSessionView {
  changed_count: number;
  display_items: WorkspaceActivityItem[];
  entries: WorkspaceFileEntry[];
  item_count: number;
  path_parts: string[];
  previewLines: string[];
  rows: FinderTreeRow[];
  selected_item: WorkspaceActivityItem | null;
  selected_entry: WorkspaceFileEntry | null;
  selected_path: string;
}

export function buildFinderSessionView({
  active_path,
  event,
  files = [],
  items,
  query = "",
  scope = "workspace",
}: {
  active_path?: string | null;
  event: NexusOperationEvent;
  files?: WorkspaceFileEntry[];
  items: WorkspaceActivityItem[];
  query?: string;
  scope?: FinderViewScope;
}): FinderSessionView {
  const display_items = items.length
    ? items
    : files.length
      ? []
      : event.kind === "workspace_search" || event.kind === "workspace_inspect"
        ? []
        : [fallback_workspace_item(active_path, event)];
  const entries = merge_workspace_entries(files, display_items, event);
  const normalized_query = query.trim().toLowerCase();
  const changed_paths = display_items
    .filter((item) => item.status !== "idle")
    .map((item) => item.path);
  const visible_entries = entries.filter((entry) => (
    (scope === "workspace" || changed_paths.some((path) => (
      paths_match(path, entry.path) || path.startsWith(`${entry.path}/`)
    )))
    && (!normalized_query || entry.path.toLowerCase().includes(normalized_query))
  ));
  const rows = workspaceTreeRows(
    visible_entries.map((entry) => entry.path),
    new Set(visible_entries.filter((entry) => entry.is_dir).map((entry) => entry.path)),
  );
  const selected_path = resolve_selected_path(active_path, event.target, rows);
  const selected_item = resolveFinderSelectedItem(display_items, selected_path);
  const selected_entry = visible_entries.find((entry) => entry.path === selected_path) ?? null;

  return {
    changed_count: new Set(display_items
      .filter((item) => item.status !== "idle")
      .map((item) => item.path)).size,
    display_items,
    entries,
    item_count: visible_entries.filter((entry) => !entry.is_dir).length,
    path_parts: selected_path.split("/").filter(Boolean),
    previewLines: finderPreviewLines(selected_item),
    rows,
    selected_entry,
    selected_item,
    selected_path,
  };
}

function fallback_workspace_item(
  active_path: string | null | undefined,
  event: NexusOperationEvent,
): WorkspaceActivityItem {
  return {
    agent_id: event.agent_id,
    event_type: "file_write_end",
    id: "empty",
    path: active_path ?? event.target ?? "workspace",
    source: "unknown",
    status: event.phase === "running" ? "writing" : "idle",
    updated_at: event.updated_at,
    version: 1,
  };
}

export function workspaceTreeRows(
  paths: string[],
  directory_paths: ReadonlySet<string> = new Set(),
): FinderTreeRow[] {
  const rows = new Map<string, FinderTreeRow>();
  paths.forEach((path) => {
    const parts = path.split("/").filter(Boolean);
    parts.forEach((part, index) => {
      const current_path = parts.slice(0, index + 1).join("/");
      const is_leaf = index === parts.length - 1;
      const type = !is_leaf || directory_paths.has(current_path) ? "folder" : "file";
      const existing = rows.get(current_path);
      if (!existing || (existing.type === "file" && type === "folder")) {
        rows.set(current_path, { depth: index, label: part, path: current_path, type });
      }
    });
  });
  return Array.from(rows.values()).sort((left, right) => {
    if (left.path === right.path) {
      return 0;
    }
    const left_parent = left.path.split("/").slice(0, -1).join("/");
    const right_parent = right.path.split("/").slice(0, -1).join("/");
    if (left_parent === right_parent && left.type !== right.type) {
      return left.type === "folder" ? -1 : 1;
    }
    return left.path.localeCompare(right.path);
  });
}

function merge_workspace_entries(
  files: WorkspaceFileEntry[],
  items: WorkspaceActivityItem[],
  event: NexusOperationEvent,
): WorkspaceFileEntry[] {
  const entries = new Map<string, WorkspaceFileEntry>();
  files.forEach((entry) => entries.set(entry.path, entry));
  items.forEach((item) => {
    if (!item.path || entries.has(item.path)) {
      return;
    }
    entries.set(item.path, {
      depth: Math.max(0, item.path.split("/").filter(Boolean).length - 1),
      is_dir: false,
      modified_at: new Date(item.updated_at).toISOString(),
      name: item.path.split("/").at(-1) ?? item.path,
      path: item.path,
    });
  });
  if (files.length === 0) {
    finderResultPaths(event.result_preview).forEach((path) => {
      if (entries.has(path)) {
        return;
      }
      entries.set(path, {
        depth: Math.max(0, path.split("/").length - 1),
        is_dir: false,
        modified_at: new Date(event.updated_at).toISOString(),
        name: path.split("/").at(-1) ?? path,
        path,
      });
    });
  }
  return Array.from(entries.values()).sort((left, right) => left.path.localeCompare(right.path));
}

function resolve_selected_path(
  active_path: string | null | undefined,
  event_target: string | null | undefined,
  rows: FinderTreeRow[],
): string {
  const candidates = [active_path, event_target].filter((value): value is string => Boolean(value));
  for (const candidate of candidates) {
    const matched_row = rows.find((row) => paths_match(row.path, candidate));
    if (matched_row) {
      return matched_row.path;
    }
  }
  return rows.find((row) => row.type === "file")?.path ?? rows[0]?.path ?? "workspace";
}

export function finderResultPaths(value: unknown): string[] {
  const paths = new Set<string>();
  collect_result_strings(value).forEach((text) => {
    text.split(/\r?\n/).forEach((line) => {
      const candidate = line.trim().replace(/^[-*]\s+/, "").replace(/:\d+(?::\d+)?:.*$/, "");
      const filename = candidate.split("/").at(-1) ?? "";
      const has_file_extension = /^\.[a-z0-9][a-z0-9._-]*$/i.test(filename)
        || /\.[a-z0-9][a-z0-9._-]*$/i.test(filename);
      if (
        !candidate
        || candidate.length > 240
        || /[{}[\]"]/u.test(candidate)
        || candidate.includes(": ")
        || /^(https?:|file:|data:|blob:)/i.test(candidate)
        || candidate.startsWith("/")
        || candidate.split("/").includes("..")
        || !has_file_extension
      ) {
        return;
      }
      paths.add(candidate.replace(/^\.\//, ""));
    });
  });
  return Array.from(paths);
}

function paths_match(left: string, right: string): boolean {
  const normalized_left = left.replace(/\\/g, "/").replace(/^\.\//, "");
  const normalized_right = right.replace(/\\/g, "/").replace(/^\.\//, "");
  return normalized_left === normalized_right
    || normalized_left.endsWith(`/${normalized_right}`)
    || normalized_right.endsWith(`/${normalized_left}`);
}

function collect_result_strings(value: unknown): string[] {
  if (typeof value === "string") {
    return [value];
  }
  if (Array.isArray(value)) {
    return value.flatMap(collect_result_strings);
  }
  if (value && typeof value === "object") {
    return Object.values(value).flatMap(collect_result_strings);
  }
  return [];
}

export function workspaceStatusLabel(status: FinderWorkspaceStatus): string {
  if (status === "writing") {
    return "写入中";
  }
  if (status === "updated") {
    return "已更新";
  }
  if (status === "deleted") {
    return "已删除";
  }
  return "未变更";
}
