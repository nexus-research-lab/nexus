"use client";

// INPUT: Structured file artifacts and the exact message/node workspace context.
// OUTPUT: Localized file cards deduplicated by resolved workspace and actual path; latest evidence wins.
// POS: Structured artifact adapter; never infers a workspace from global Agent selection or hides evidence without preview.

import type { WorkspaceFileOpenHandler } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import type { WorkspaceFileArtifactContent } from "@/types/conversation/message/content";

import { buildWorkspaceFileArtifactEntries } from "./workspace-file-artifact-list-model";
import { FileArtifactBlock } from "./file/file-artifact-block";
import { firstNonEmptyArtifactValue } from "./artifact-path-model";

interface WorkspaceFileArtifactListProps {
  artifacts: WorkspaceFileArtifactContent[];
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  workspaceAgentId?: string | null;
  label?: string;
  className?: string;
}

interface WorkspaceFileArtifactBlockProps {
  artifact: WorkspaceFileArtifactContent;
  onOpenWorkspaceFile?: WorkspaceFileOpenHandler;
  workspaceAgentId?: string | null;
  compact?: boolean;
  className?: string;
}

export function WorkspaceFileArtifactBlock({
  artifact,
  onOpenWorkspaceFile,
  workspaceAgentId,
  compact = false,
  className,
}: WorkspaceFileArtifactBlockProps) {
  const { t } = useI18n();
  return (
    <FileArtifactBlock
      compact={compact}
      className={className}
      label={artifact.label ?? t("workspace_file.default_name")}
      path={artifact.path}
      displayPath={artifact.display_path ?? artifact.path}
      workspaceAgentId={firstNonEmptyArtifactValue(artifact.workspace_agent_id, workspaceAgentId)}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}

export function WorkspaceFileArtifactList({
  artifacts,
  onOpenWorkspaceFile,
  workspaceAgentId,
  label,
  className,
}: WorkspaceFileArtifactListProps) {
  const { t } = useI18n();
  const visibleLabel = label ?? t("message.generated_files");
  const entries = buildWorkspaceFileArtifactEntries(artifacts, workspaceAgentId);
  if (entries.length === 0) {
    return null;
  }

  return (
    <div className={cn("min-w-0 space-y-1.5", className)}>
      {visibleLabel ? (
        <div className={getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" })}>
          {visibleLabel}
        </div>
      ) : null}
      <div className="min-w-0 space-y-1.5">
        {entries.map(({ key, artifact }) => (
          <WorkspaceFileArtifactBlock
            key={key}
            compact
            artifact={{ ...artifact, label: "" }}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            workspaceAgentId={workspaceAgentId}
          />
        ))}
      </div>
    </div>
  );
}
