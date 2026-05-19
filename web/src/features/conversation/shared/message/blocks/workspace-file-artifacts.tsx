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

function artifact_key(artifact: WorkspaceFileArtifactContent): string {
  return (
    artifact.id ||
    `${artifact.source_tool_use_id ?? "workspace_file"}:${artifact.path}`
  );
}

export function WorkspaceFileArtifactBlock({
  artifact,
  on_open_workspace_file,
  compact = false,
  class_name,
}: WorkspaceFileArtifactBlockProps) {
  return (
    <FileArtifactBlock
      compact={compact}
      class_name={class_name}
      label={artifact.label ?? "文件"}
      path={artifact.path}
      display_path={artifact.display_path ?? artifact.path}
      on_open_workspace_file={on_open_workspace_file}
    />
  );
}

export function WorkspaceFileArtifactList({
  artifacts,
  on_open_workspace_file,
  label = "生成文件",
  class_name,
}: WorkspaceFileArtifactListProps) {
  if (!on_open_workspace_file || artifacts.length === 0) {
    return null;
  }

  return (
    <div className={cn("min-w-0 space-y-1.5", class_name)}>
      {label ? (
        <div className="text-[11px] font-medium leading-4 text-(--text-muted)">
          {label}
        </div>
      ) : null}
      <div className="min-w-0 space-y-1.5">
        {artifacts.map((artifact) => (
          <WorkspaceFileArtifactBlock
            key={artifact_key(artifact)}
            compact
            artifact={{ ...artifact, label: "" }}
            on_open_workspace_file={on_open_workspace_file}
          />
        ))}
      </div>
    </div>
  );
}
