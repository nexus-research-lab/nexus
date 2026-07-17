/**
 * INPUT: One workspace tree row with optional file metadata and live activity.
 * OUTPUT: Keyboard-accessible Finder-like file or folder interaction.
 * POS: Files list presentation boundary shared by the workspace surface.
 */
import {
  ChevronDown,
  ChevronRight,
  FolderOpen,
  Tag,
} from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import type { WorkspaceFileEntry } from "@/types/agent/agent";
import type { WorkspaceActivityItem } from "@/types/app/workspace-live";

import { workspaceStatusLabel } from "./finder-session";
import {
  formatWorkspaceFileTime,
  workspaceFileIcon,
} from "./workspace-finder-presentation";

export function WorkspaceFinderRow({
  active,
  collapsed,
  compact,
  depth,
  entry,
  item,
  label,
  onOpen,
  onSelect,
  onToggle,
  path,
  type,
}: {
  active: boolean;
  collapsed: boolean;
  compact: boolean;
  depth: number;
  entry?: WorkspaceFileEntry;
  item?: WorkspaceActivityItem;
  label: string;
  onOpen?: () => void;
  onSelect: () => void;
  onToggle: () => void;
  path: string;
  type: "folder" | "file";
}) {
  const status = item?.status;
  const Icon = type === "folder" ? FolderOpen : workspaceFileIcon(path);
  const open_row = () => {
    if (type === "folder") {
      onToggle();
    } else if (status !== "deleted") {
      onOpen?.();
    }
  };
  return (
    <button
      aria-label={type === "folder" ? `${collapsed ? "展开" : "收起"} ${label}` : `打开 ${label}`}
      className={cn(
        "grid w-full items-center gap-2 rounded-[9px] px-2 py-1.5 text-left text-[11px] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]",
        compact
          ? "grid-cols-[auto_auto_auto_minmax(0,1fr)]"
          : "grid-cols-[auto_auto_auto_minmax(0,1fr)_72px_86px]",
        active ? "bg-[rgba(91,114,255,0.12)] text-[color:var(--primary)]" : "text-(--text-muted) hover:bg-white/70",
      )}
      onClick={onSelect}
      onDoubleClick={open_row}
      onKeyDown={(keyboard_event) => {
        if (keyboard_event.key !== "Enter") {
          return;
        }
        keyboard_event.preventDefault();
        open_row();
      }}
      title={path}
      type="button"
    >
      <span style={{ width: depth * 12 }} className="shrink-0" />
      {type === "folder" ? (
        collapsed
          ? <ChevronRight className="h-3 w-3 shrink-0 text-(--icon-muted)" />
          : <ChevronDown className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      ) : (
        <span className="h-3 w-3 shrink-0" />
      )}
      <Icon className="h-3.5 w-3.5 shrink-0" />
      <span className={cn("min-w-0 flex-1 truncate", type === "folder" && "font-bold text-(--text-strong)")}>
        {label}
      </span>
      {!compact && status ? (
        <span className="inline-flex shrink-0 items-center gap-1 text-[9px] font-bold text-(--text-soft)">
          <Tag className={cn(
            "h-2.5 w-2.5 fill-current",
            status === "writing" && "text-[color:var(--primary)]",
            status === "updated" && "text-[color:var(--success)]",
            status === "deleted" && "text-[color:var(--destructive)]",
            status === "idle" && "text-(--icon-muted)",
          )} />
          {workspaceStatusLabel(status)}
        </span>
      ) : compact ? null : <span />}
      {compact ? null : (
        <span className="truncate text-[9px] text-(--text-soft)">
          {formatWorkspaceFileTime(entry?.modified_at ?? item?.updated_at)}
        </span>
      )}
    </button>
  );
}
