"use client";

import { FileText, FolderOpen } from "lucide-react";

import { cn } from "@/lib/utils";

interface FileArtifactBlockProps {
  label?: string;
  path: string;
  display_path?: string;
  on_open_workspace_file?: (path: string) => void;
  compact?: boolean;
  class_name?: string;
}

function file_name_from_path(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).at(-1) ?? normalized;
}

function file_parent_from_path(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  const parts = normalized.split("/").filter(Boolean);
  if (parts.length <= 1) {
    return "workspace";
  }
  return parts.slice(0, -1).join("/");
}

export function FileArtifactBlock({
  label = "已保存到",
  path,
  display_path,
  on_open_workspace_file,
  compact = false,
  class_name,
}: FileArtifactBlockProps) {
  const display_path_value = display_path?.trim() || path;
  const file_name = file_name_from_path(display_path_value);
  const parent_path = file_parent_from_path(display_path_value);
  const can_open = Boolean(on_open_workspace_file);

  return (
    <div className={cn(compact ? "my-0" : "my-2", "min-w-0", class_name)}>
      {label ? (
        <div className={cn("mb-1 text-(--text-default)", compact ? "text-[12px] leading-5" : "text-[14px] leading-6")}>
          {label}
        </div>
      ) : null}
      <button
        type="button"
        title={path}
        disabled={!can_open}
        onClick={() => on_open_workspace_file?.(path)}
        className={cn(
          "group flex w-full min-w-0 items-center rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-left shadow-[0_1px_0_rgba(0,0,0,0.03)] transition-colors",
          compact
            ? "max-w-[28rem] gap-2 px-2.5 py-2"
            : "max-w-[32rem] gap-3 px-3 py-2.5",
          can_open
            ? "cursor-pointer hover:border-primary/30 hover:bg-primary/5"
            : "cursor-default opacity-80",
        )}
      >
        <span
          className={cn(
            "flex shrink-0 items-center justify-center rounded-[7px] border border-primary/15 bg-primary/8 text-primary",
            compact ? "h-8 w-8" : "h-9 w-9",
          )}
        >
          <FileText className={compact ? "h-3.5 w-3.5" : "h-4 w-4"} />
        </span>
        <span className="min-w-0 flex-1">
          <span className={cn("message-cjk-code-font block truncate font-medium text-(--text-strong)", compact ? "text-[13px] leading-5" : "text-[14px] leading-5")}>
            {file_name}
          </span>
          <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[12px] leading-4 text-(--text-muted)">
            <FolderOpen className="h-3 w-3 shrink-0 text-(--icon-muted)" />
            <span className="truncate">{parent_path}</span>
          </span>
        </span>
        <span className={cn("shrink-0 rounded-[6px] border border-primary/15 bg-primary/8 font-medium text-primary transition-colors group-hover:bg-primary/12", compact ? "px-1.5 py-0.5 text-[10px]" : "px-2 py-1 text-[11px]")}>
          打开
        </span>
      </button>
    </div>
  );
}
