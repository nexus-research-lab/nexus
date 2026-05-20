"use client";

import { Download, FileText, FolderOpen } from "lucide-react";

import { get_workspace_file_download_url } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { useAgentStore } from "@/store/agent";

interface FileArtifactBlockProps {
  label?: string;
  path: string;
  display_path?: string;
  on_open_workspace_file?: (path: string) => void;
  workspace_agent_id?: string | null;
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
  workspace_agent_id,
  compact = false,
  class_name,
}: FileArtifactBlockProps) {
  const current_agent_id = useAgentStore((state) => state.current_agent_id);
  const display_path_value = display_path?.trim() || path;
  const file_name = file_name_from_path(display_path_value);
  const parent_path = file_parent_from_path(display_path_value);
  const can_open = Boolean(on_open_workspace_file);
  const download_agent_id = workspace_agent_id?.trim() || current_agent_id || "";
  const can_download = Boolean(download_agent_id && path.trim());
  const download_url = can_download ? get_workspace_file_download_url(download_agent_id, path) : "";

  return (
    <div className={cn(compact ? "my-0" : "my-2", "min-w-0", class_name)}>
      {label ? (
        <div className={cn("mb-1 text-(--text-default)", compact ? "text-[12px] leading-5" : "text-[14px] leading-6")}>
          {label}
        </div>
      ) : null}
      <div
        className={cn(
          "group flex w-full min-w-0 items-center rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-left shadow-[0_1px_0_rgba(0,0,0,0.03)] transition-colors",
          compact
            ? "max-w-[28rem] gap-1.5 px-2.5 py-2"
            : "max-w-[32rem] gap-2 px-3 py-2.5",
          can_open || can_download ? "hover:border-primary/30 hover:bg-primary/5" : "opacity-80",
        )}
      >
        <button
          className="flex min-w-0 flex-1 items-center gap-2 text-left disabled:cursor-default"
          disabled={!can_open}
          onClick={() => on_open_workspace_file?.(path)}
          title={path}
          type="button"
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
          {can_open ? (
            <span className={cn("shrink-0 rounded-[6px] border border-primary/15 bg-primary/8 font-medium text-primary transition-colors group-hover:bg-primary/12", compact ? "px-1.5 py-0.5 text-[10px]" : "px-2 py-1 text-[11px]")}>
              打开
            </span>
          ) : null}
        </button>
        {can_download ? (
          <a
            aria-label={`下载 ${file_name}`}
            className={cn(
              "inline-flex shrink-0 items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) text-(--text-muted) transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-primary",
              compact ? "px-1.5 py-1 text-[10px]" : "px-2 py-1 text-[11px]",
            )}
            download={file_name}
            href={download_url}
            rel="noopener noreferrer"
            target="_blank"
            title={`下载 ${file_name}`}
          >
            <Download className={compact ? "h-3 w-3" : "h-3.5 w-3.5"} />
            <span>下载</span>
          </a>
        ) : null}
      </div>
    </div>
  );
}
