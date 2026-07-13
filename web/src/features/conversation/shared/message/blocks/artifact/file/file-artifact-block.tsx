"use client";

import { memo } from "react";
import { FileText, FolderOpen } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useAgentStore } from "@/store/agent";

import { WorkspaceArtifactExternalActionButton } from "../workspace-artifact-external-action";
import {
  type FileArtifactProjection,
  projectFileArtifact,
} from "./file-artifact-model";

interface FileArtifactBlockProps {
  className?: string;
  compact?: boolean;
  displayPath?: string;
  label?: string;
  onOpenWorkspaceFile?: (
    path: string,
    workspaceAgentId?: string | null,
  ) => void;
  path: string;
  workspaceAgentId?: string | null;
}

function FileArtifactBlockComponent({
  className,
  compact = false,
  displayPath,
  label = "已保存到",
  onOpenWorkspaceFile,
  path,
  workspaceAgentId,
}: FileArtifactBlockProps) {
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const projection = projectFileArtifact({
    compact,
    currentAgentId,
    displayPath,
    hasOpenHandler: Boolean(onOpenWorkspaceFile),
    path,
    workspaceAgentId,
  });
  return (
    <div className={cn(projection.style.wrapper, "min-w-0", className)}>
      <FileArtifactLabel
        className={projection.style.label}
        label={label}
      />
      <div
        className={cn(
          "group flex w-full min-w-0 items-center rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background) text-left shadow-[0_1px_0_rgba(0,0,0,0.03)] transition-colors",
          projection.style.card,
          [projection.canOpen, Boolean(projection.action)].some(Boolean)
            ? "hover:border-primary/30 hover:bg-primary/5"
            : "opacity-80",
        )}
      >
        <FileArtifactOpenButton
          onOpen={() => onOpenWorkspaceFile?.(path, projection.openAgentId)}
          path={path}
          projection={projection}
        />
        <WorkspaceArtifactExternalActionButton
          action={projection.action}
          className={cn(
            "inline-flex shrink-0 items-center gap-1 rounded-[6px] border border-(--divider-subtle-color) text-(--text-muted) transition-colors hover:border-primary/25 hover:bg-primary/8 hover:text-primary",
            projection.style.externalAction,
          )}
          iconClassName={projection.style.externalIcon}
        />
      </div>
    </div>
  );
}

function FileArtifactLabel({
  className,
  label,
}: {
  className: string;
  label: string;
}) {
  if (!label) {
    return null;
  }
  return (
    <div className={cn("mb-1 text-(--text-default)", className)}>
      {label}
    </div>
  );
}

function FileArtifactOpenButton({
  onOpen,
  path,
  projection,
}: {
  onOpen: () => void;
  path: string;
  projection: FileArtifactProjection;
}) {
  return (
    <button
      className="flex min-w-0 flex-1 items-center gap-2 text-left disabled:cursor-default"
      disabled={!projection.canOpen}
      onClick={onOpen}
      title={path}
      type="button"
    >
      <span
        className={cn(
          "flex shrink-0 items-center justify-center rounded-[7px] border border-primary/15 bg-primary/8 text-primary",
          projection.style.iconFrame,
        )}
      >
        <FileText className={projection.style.fileIcon} />
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            "message-cjk-code-font block truncate font-medium text-(--text-strong)",
            projection.style.fileName,
          )}
        >
          {projection.fileName}
        </span>
        <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[12px] leading-4 text-(--text-muted)">
          <FolderOpen className="h-3 w-3 shrink-0 text-(--icon-muted)" />
          <span className="truncate">{projection.parentPath}</span>
        </span>
      </span>
      <FileArtifactOpenBadge
        className={projection.style.openBadge}
        visible={projection.canOpen}
      />
    </button>
  );
}

function FileArtifactOpenBadge({
  className,
  visible,
}: {
  className: string;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <span
      className={cn(
        "shrink-0 rounded-[6px] border border-primary/15 bg-primary/8 font-medium text-primary transition-colors group-hover:bg-primary/12",
        className,
      )}
    >
      打开
    </span>
  );
}

export const FileArtifactBlock = memo(FileArtifactBlockComponent);
