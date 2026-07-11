import type { RefObject } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";

import type {
  ContentBlock,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message/content";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import { useWorkspaceFileArtifactsFromContent } from "../../../blocks/artifact/workspace-file-artifact-utils";
import { ContentRenderer } from "../content/content-renderer";
import type {
  AssistantActivityState,
  AssistantContentEnvironment,
  AssistantPermissionState,
  AssistantProcessState,
} from "./assistant-message-model";

const EMPTY_CONTENT_BLOCKS: ContentBlock[] = [];

interface AssistantProcessCallchainProps {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
}

export function AssistantProcessCallchain({
  activity,
  environment,
  permissions,
  process,
}: AssistantProcessCallchainProps) {
  const collapsedFileArtifacts = useWorkspaceFileArtifactsFromContent(
    selectCollapsedProcessContent(process),
  );

  if (!process.visible) {
    return null;
  }

  return (
    <div ref={process.anchorRef as RefObject<HTMLDivElement>}>
      <ProcessToggleButton process={process} />
      <CollapsedProcessArtifacts
        artifacts={collapsedFileArtifacts}
        onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
        visible={!process.expanded}
      />
      <ExpandedProcessContent
        activity={activity}
        environment={environment}
        permissions={permissions}
        process={process}
        visible={process.expanded}
      />
    </div>
  );
}

function selectCollapsedProcessContent(
  process: AssistantProcessState,
): ContentBlock[] {
  const shouldCollectArtifacts = process.visible && !process.expanded;
  return shouldCollectArtifacts
    ? process.projection.content
    : EMPTY_CONTENT_BLOCKS;
}

function ProcessToggleButton({ process }: { process: AssistantProcessState }) {
  return (
    <button
      className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
      onClick={process.toggle}
      type="button"
    >
      <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
      <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-muted)">
        {process.summary}
      </div>
      <ProcessExpansionIcon expanded={process.expanded} />
    </button>
  );
}

function ProcessExpansionIcon({ expanded }: { expanded: boolean }) {
  const Icon = expanded ? ChevronDown : ChevronRight;
  return (
    <div className="text-(--icon-muted)">
      <Icon className="h-3.5 w-3.5" />
    </div>
  );
}

function CollapsedProcessArtifacts({
  artifacts,
  onOpenWorkspaceFile,
  visible,
}: {
  artifacts: WorkspaceFileArtifactContent[];
  onOpenWorkspaceFile?: (path: string) => void;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <WorkspaceFileArtifactList
      artifacts={artifacts}
      className="ml-5 pb-1"
      label="生成文件"
      onOpenWorkspaceFile={onOpenWorkspaceFile}
    />
  );
}

function ExpandedProcessContent({
  activity,
  environment,
  permissions,
  process,
  visible,
}: {
  activity: AssistantActivityState;
  environment: AssistantContentEnvironment;
  permissions: AssistantPermissionState;
  process: AssistantProcessState;
  visible: boolean;
}) {
  if (!visible) {
    return null;
  }
  return (
    <div className="pt-1">
      <ContentRenderer
        canRespondToPermissions={environment.canRespondToPermissions}
        className="ml-1"
        content={process.projection.content}
        fallbackActivityState={activity.state}
        hiddenToolNames={environment.hiddenToolNames}
        isStreaming={activity.showCursor}
        onOpenWorkspaceFile={environment.onOpenWorkspaceFile}
        onPermissionResponse={environment.onPermissionResponse}
        pendingPermissionsByToolUseId={permissions.matchedByToolUseId}
        permissionReadOnlyReason={environment.permissionReadOnlyReason}
        showTimelineDots
        streamingBlockIndexes={process.projection.streamingIndexes}
        workspaceAgentId={environment.workspaceAgentId}
      />
    </div>
  );
}
