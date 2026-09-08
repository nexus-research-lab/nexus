"use client";

// INPUT: 文件 Artifact 协议、workspace 身份与紧凑展示开关。
// OUTPUT: 文件身份、共享来源资格的预览/外部动作与缺失身份说明；不读取全局 Agent。
// POS: File Artifact 视图；纯模型只投影身份与资格，内容几何来自 file-artifact-layout。

import { memo, useId } from "react";
import { FileText, FolderOpen } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { resolveFileArtifactLayout } from "./file-artifact-layout";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";

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
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
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
  const unavailableId = useId();
  const projection = projectFileArtifact({
    defaultFileName: t("workspace_file.default_name"),
    displayPath,
    hasOpenHandler: Boolean(onOpenWorkspaceFile),
    path,
    workspaceAgentId,
  });
  const actionable = projection.action !== null;
  const unavailableText = projection.unavailableReason === "path"
    ? t("workspace_file.path_unavailable")
    : projection.unavailableReason === "workspace"
      ? t("workspace_file.workspace_unavailable")
      : null;
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
          describedBy={unavailableText ? unavailableId : undefined}
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
      {unavailableText ? (
        <p className={cn("mt-1", getUiTypographyClassName({ role: "metadata", tone: "muted" }))} id={unavailableId}>
          {unavailableText}
        </p>
      ) : null}
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
  describedBy,
  layout,
  onOpen,
  path,
  projection,
}: {
  describedBy?: string;
  onOpen: () => void;
  path: string;
  projection: FileArtifactProjection;
  layout: ReturnType<typeof resolveFileArtifactLayout>;
}) {
  return (
    <button
      aria-describedby={describedBy}
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
        <FileText aria-hidden="true" className={layout.fileIcon} />
      </span>
      <span className="min-w-0 flex-1">
        <span
          className={cn(
            "message-cjk-code-font block truncate",
            layout.fileName,
          )}
        >
          {projection.fileName}
        </span>
        <span className={cn("mt-0.5 flex min-w-0 items-center gap-1.5", getUiTypographyClassName({ role: "metadata", tone: "muted" }))}>
          <FolderOpen aria-hidden="true" className="h-3 w-3 shrink-0 text-(--icon-muted)" />
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
