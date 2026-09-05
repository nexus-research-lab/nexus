"use client";

// INPUT: 文件 Artifact 协议、workspace 身份与紧凑展示开关。
// OUTPUT: 按领域阅读布局组合文件身份、原 scope 打开和独立外部动作。
// POS: File Artifact 视图；纯模型只投影身份与资格，内容几何来自 file-artifact-layout。

import { memo } from "react";
import { FileText, FolderOpen } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { resolveFileArtifactLayout } from "./file-artifact-layout";
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
  label,
  onOpenWorkspaceFile,
  path,
  workspaceAgentId,
}: FileArtifactBlockProps) {
  const { t } = useI18n();
  const layout = resolveFileArtifactLayout(compact);
  const currentAgentId = useAgentStore((state) => state.current_agent_id);
  const projection = projectFileArtifact({
    currentAgentId,
    displayPath,
    hasOpenHandler: Boolean(onOpenWorkspaceFile),
    path,
    workspaceAgentId,
  });
  const actionable = [projection.canOpen, Boolean(projection.action)].some(Boolean);
  return (
    <div className={cn(layout.wrapper, "min-w-0", className)}>
      <FileArtifactLabel
        className={layout.label}
        label={label ?? t("workspace_file.saved_to")}
      />
      <div
        className={cn(
          "content-artifact-row group flex w-full min-w-0 items-center text-left",
          layout.card,
          !actionable && "opacity-80",
        )}
        data-actionable={actionable ? "true" : undefined}
      >
        <FileArtifactOpenButton
          layout={layout}
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
  layout,
  onOpen,
  path,
  projection,
}: {
  onOpen: () => void;
  path: string;
  projection: FileArtifactProjection;
  layout: ReturnType<typeof resolveFileArtifactLayout>;
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
          layout.iconFrame,
        )}
      >
        <FileText className={layout.fileIcon} />
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            "message-cjk-code-font block truncate font-medium text-(--text-strong)",
            layout.fileName,
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
        className={layout.openBadge}
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
  const { t } = useI18n();
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
      {t("workspace_file.open")}
    </span>
  );
}

export const FileArtifactBlock = memo(FileArtifactBlockComponent);
