"use client";

import { FileText, FolderOpen } from "lucide-react";

import { cn } from "@/lib/utils";

interface FileArtifactBlockProps {
  label?: string;
  path: string;
  display_path?: string;
  on_open_workspace_file?: (path: string) => void;
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
}: FileArtifactBlockProps) {
  const display_path_value = display_path?.trim() || path;
  const file_name = file_name_from_path(display_path_value);
  const parent_path = file_parent_from_path(display_path_value);
  const can_open = Boolean(on_open_workspace_file);

  return (
    <div className="my-2 min-w-0">
      {label ? (
        <div className="mb-1 text-[14px] leading-6 text-(--text-default)">
          {label}
        </div>
      ) : null}
      <button
        type="button"
        title={path}
        disabled={!can_open}
        onClick={() => on_open_workspace_file?.(path)}
        className={cn(
          "group flex w-full max-w-[32rem] min-w-0 items-center gap-3 rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-2.5 text-left shadow-[0_1px_0_rgba(0,0,0,0.03)] transition-colors",
          can_open
            ? "cursor-pointer hover:border-primary/30 hover:bg-primary/5"
            : "cursor-default opacity-80",
        )}
      >
        <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[7px] border border-primary/15 bg-primary/8 text-primary">
          <FileText className="h-4 w-4" />
        </span>
        <span className="min-w-0 flex-1">
          <span className="message-cjk-code-font block truncate text-[14px] font-medium leading-5 text-(--text-strong)">
            {file_name}
          </span>
          <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[12px] leading-4 text-(--text-muted)">
            <FolderOpen className="h-3 w-3 shrink-0 text-(--icon-muted)" />
            <span className="truncate">{parent_path}</span>
          </span>
        </span>
        <span className="shrink-0 rounded-[6px] border border-primary/15 bg-primary/8 px-2 py-1 text-[11px] font-medium text-primary transition-colors group-hover:bg-primary/12">
          打开
        </span>
      </button>
    </div>
  );
}
