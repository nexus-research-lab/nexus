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
  const actionable = [projection.canOpen, Boolean(projection.action)].some(Boolean);
  return (
    <div className={cn(projection.style.wrapper, "min-w-0", className)}>
      <FileArtifactLabel
        className={projection.style.label}
        label={label}
      />
      <div
        className={cn(
          "content-artifact-row group flex w-full min-w-0 items-center text-left",
          projection.style.card,
          !actionable && "opacity-80",
        )}
        data-actionable={actionable ? "true" : undefined}
      >
        <FileArtifactOpenButton
          onOpen={() => onOpenWorkspaceFile?.(path, projection.openAgentId)}
          path={path}
          projection={projection}
        />
        <WorkspaceArtifactExternalActionButton
          action={projection.action}
          className="content-artifact-external-action shrink-0"
          size={compact ? "2xs" : "xs"}
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
          "content-artifact-icon",
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
        <span className="mt-0.5 flex min-w-0 items-center gap-1.5 text-compact leading-4 text-(--text-muted)">
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
        "content-artifact-open shrink-0 font-medium",
        className,
      )}
    >
      打开
    </span>
  );
}

export const FileArtifactBlock = memo(FileArtifactBlockComponent);
