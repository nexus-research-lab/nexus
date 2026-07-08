"use client";

import { Download, FileText, FolderOpen } from "lucide-react";

import {
  downloadWorkspaceFileApi,
} from "@/lib/api/agent-manage-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { cn } from "@/lib/utils";
import { useAgentStore } from "@/store/agent";

interface FileArtifactBlockProps {
  label?: string;
  path: string;
  displayPath?: string;
  onOpenWorkspaceFile?: (path: string) => void;
  workspaceAgentId?: string | null;
  compact?: boolean;
  className?: string;
}

function fileNameFromPath(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/");
  return normalized.split("/").filter(Boolean).at(-1) ?? normalized;
}

function fileParentFromPath(path: string): string {
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
  displayPath: displayPath,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  workspaceAgentId: workspaceAgentId,
  compact = false,
  className: className,
}: FileArtifactBlockProps) {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const displayPathValue = displayPath?.trim() || path;
  const fileName = fileNameFromPath(displayPathValue);
  const parentPath = fileParentFromPath(displayPathValue);
  const canOpen = Boolean(onOpenWorkspaceFile);
  const downloadAgentId = workspaceAgentId?.trim() || currentAgentId || "";
  const canDownload = Boolean(downloadAgentId && path.trim());
  const fileActionCopy = getWorkspaceFileExternalActionCopy(fileName);
  const handleExternalAction = () => {
    if (!canDownload) {
      return;
    }
    void downloadWorkspaceFileApi(downloadAgentId, path, fileName).catch((error) => {
      console.error(`[FileArtifactBlock] ${fileActionCopy.label} workspace 文件失败:`, error);
    });
  };

  return (
    <div className={cn(compact ? "my-0" : "my-2", "min-w-0", className)}>
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
          canOpen || canDownload ? "hover:border-primary/30 hover:bg-primary/5" : "opacity-80",
        )}
      >
        <button
          className="flex min-w-0 flex-1 items-center gap-2 text-left disabled:cursor-default"
          disabled={!canOpen}
          onClick={() => onOpenWorkspaceFile?.(path)}
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
              {fileName}
            </span>
            <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[12px] leading-4 text-(--text-muted)">
              <FolderOpen className="h-3 w-3 shrink-0 text-(--icon-muted)" />
              <span className="truncate">{parentPath}</span>
            </span>
          </span>
          {canOpen ? (
            <span className={cn("shrink-0 rounded-[6px] border border-primary/15 bg-primary/8 font-medium text-primary transition-colors group-hover:bg-primary/12", compact ? "px-1.5 py-0.5 text-[10px]" : "px-2 py-1 text-[11px]")}>
              打开
            </span>
          ) : null}
        </button>
        {canDownload ? (
          <button
            aria-label={fileActionCopy.ariaLabel}
            className={cn(
              "inline-flex shrink-0 items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) text-(--text-muted) transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-primary",
              compact ? "px-1.5 py-1 text-[10px]" : "px-2 py-1 text-[11px]",
            )}
            onClick={handleExternalAction}
            title={fileActionCopy.title}
            type="button"
          >
            {fileActionCopy.mode === "reveal" ? (
              <FolderOpen className={compact ? "h-3 w-3" : "h-3.5 w-3.5"} />
            ) : (
              <Download className={compact ? "h-3 w-3" : "h-3.5 w-3.5"} />
            )}
            <span>{fileActionCopy.label}</span>
          </button>
        ) : null}
      </div>
    </div>
  );
}
