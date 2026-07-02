"use client";

import { cn } from "@/lib/utils";
import type { WorkspaceFileArtifactContent } from "@/types/conversation/message";

import { FileArtifactBlock } from "./file-artifact-block";

interface WorkspaceFileArtifactListProps {
  artifacts: WorkspaceFileArtifactContent[];
  on_open_workspace_file?: (path: string) => void;
  label?: string;
  class_name?: string;
}

interface WorkspaceFileArtifactBlockProps {
  artifact: WorkspaceFileArtifactContent;
  on_open_workspace_file?: (path: string) => void;
  compact?: boolean;
  class_name?: string;
}

function artifactKey(artifact: WorkspaceFileArtifactContent): string {
  return (
    artifact.id ||
    `${artifact.source_tool_use_id ?? "workspace_file"}:${artifact.path}`
  );
}

export function WorkspaceFileArtifactBlock({
  artifact,
  on_open_workspace_file: onOpenWorkspaceFile,
  compact = false,
  class_name: className,
}: WorkspaceFileArtifactBlockProps) {
  return (
    <FileArtifactBlock
      compact={compact}
      class_name={className}
      label={artifact.label ?? "文件"}
      path={artifact.path}
      display_path={artifact.display_path ?? artifact.path}
      workspace_agent_id={artifact.workspace_agent_id}
      on_open_workspace_file={onOpenWorkspaceFile}
    />
  );
}

export function WorkspaceFileArtifactList({
  artifacts,
  on_open_workspace_file: onOpenWorkspaceFile,
  label = "生成文件",
  class_name: className,
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
            on_open_workspace_file={onOpenWorkspaceFile}
          />
        ))}
      </div>
    </div>
  );
}
