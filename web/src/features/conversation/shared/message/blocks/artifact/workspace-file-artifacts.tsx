"use client";

import { cn } from "@/shared/ui/class-name";
import type { WorkspaceFileArtifactContent } from "@/types/conversation/message/content";

import { FileArtifactBlock } from "./file-artifact-block";

interface WorkspaceFileArtifactListProps {
  artifacts: WorkspaceFileArtifactContent[];
  onOpenWorkspaceFile?: (path: string) => void;
  label?: string;
  className?: string;
}

interface WorkspaceFileArtifactBlockProps {
  artifact: WorkspaceFileArtifactContent;
  onOpenWorkspaceFile?: (path: string) => void;
  compact?: boolean;
  className?: string;
}

function artifactKey(artifact: WorkspaceFileArtifactContent): string {
  return (
    artifact.id ||
    `${artifact.source_tool_use_id ?? "workspace_file"}:${artifact.path}`
  );
}

export function WorkspaceFileArtifactBlock({
  artifact,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  compact = false,
  className: className,
}: WorkspaceFileArtifactBlockProps) {
  return (
    <FileArtifactBlock
      compact={compact}
      className={className}
      label={artifact.label ?? "文件"}
      path={artifact.path}
      displayPath={artifact.display_path ?? artifact.path}
      workspaceAgentId={artifact.workspace_agent_id}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}

export function WorkspaceFileArtifactList({
  artifacts,
  onOpenWorkspaceFile: onOpenWorkspaceFile,
  label = "生成文件",
  className: className,
}: WorkspaceFileArtifactListProps) {
  if (!onOpenWorkspaceFile || artifacts.length === 0) {
    return null;
  }

  return (
    <div className={cn("min-w-0 space-y-1.5", className)}>
      {label ? (
        <div className="text-[11px] font-medium leading-4 text-(--text-muted)">
          {label}
        </div>
      ) : null}
      <div className="min-w-0 space-y-1.5">
        {artifacts.map((artifact) => (
          <WorkspaceFileArtifactBlock
            key={artifactKey(artifact)}
            compact
            artifact={{ ...artifact, label: "" }}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
          />
        ))}
      </div>
    </div>
  );
}
