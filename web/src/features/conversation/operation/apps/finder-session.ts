import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

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

export interface FinderSessionView {
  changed_count: number;
  display_items: WorkspaceActivityItem[];
  item_count: number;
  path_parts: string[];
  previewLines: string[];
  rows: FinderTreeRow[];
  selected_item: WorkspaceActivityItem | null;
  selected_path: string;
}

export function buildFinderSessionView({
  active_path,
  event,
  items,
}: {
  active_path?: string | null;
  event: NexusOperationEvent;
  items: WorkspaceActivityItem[];
}): FinderSessionView {
  const display_items = items.length ? items : [fallback_workspace_item(active_path, event)];
  const selected_path = active_path ?? event.target ?? display_items[0]?.path ?? "workspace";
  const selected_item = resolveFinderSelectedItem(display_items, selected_path);

  return {
    changed_count: display_items.filter((item) => item.status === "updated" || item.status === "writing").length,
    display_items,
    item_count: display_items.length,
    path_parts: selected_path.split("/").filter(Boolean),
    previewLines: finderPreviewLines(selected_item),
    rows: workspaceTreeRows(display_items.map((item) => item.path)),
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

export function workspaceTreeRows(paths: string[]): FinderTreeRow[] {
  const rows = new Map<string, FinderTreeRow>();
  paths.forEach((path) => {
    const parts = path.split("/").filter(Boolean);
    parts.forEach((part, index) => {
      const current_path = parts.slice(0, index + 1).join("/");
      if (!rows.has(current_path)) {
        rows.set(current_path, {
          depth: index,
          label: part,
          path: current_path,
          type: index === parts.length - 1 ? "file" : "folder",
        });
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
