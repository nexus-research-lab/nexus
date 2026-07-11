import type { ReactNode, RefObject } from "react";
import { ChevronDown, ChevronRight, Wrench } from "lucide-react";

import type { WorkspaceFileArtifactContent } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

import { WorkspaceFileArtifactList } from "../../../blocks/artifact/workspace-file-artifacts";
import type { MessageActivityState } from "../../../ui/message-primitives";
import type { ContentProjection } from "../../message-item-projection";
import { ContentRenderer } from "../content/content-renderer";

interface AssistantProcessCallchainProps {
  anchorRef: RefObject<HTMLElement | null>;
  canRespondToPermissions: boolean;
  collapsedFileArtifacts: WorkspaceFileArtifactContent[];
  fallbackActivityState: MessageActivityState | null;
  hiddenToolNames: string[];
  isExpanded: boolean;
  isStreaming: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionBlock: ReactNode;
  pendingPermissionsByToolUseId: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  processProjection: ContentProjection;
  summary: string;
  toggleExpanded: () => void;
  workspaceAgentId?: string | null;
}

export function AssistantProcessCallchain({
  anchorRef,
  canRespondToPermissions,
  collapsedFileArtifacts,
  fallbackActivityState,
  hiddenToolNames,
  isExpanded,
  isStreaming,
  onOpenWorkspaceFile,
  onPermissionResponse,
  pendingPermissionBlock,
  pendingPermissionsByToolUseId,
  permissionReadOnlyReason,
  processProjection,
  summary,
  toggleExpanded,
  workspaceAgentId,
}: AssistantProcessCallchainProps) {
  return (
    <div ref={anchorRef as RefObject<HTMLDivElement>}>
      <button
        className="flex w-full items-center gap-2 py-1.5 text-left text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong)"
        onClick={toggleExpanded}
        type="button"
      >
        <Wrench className="h-3 w-3 shrink-0 text-(--icon-muted)" />
        <div className="min-w-0 flex-1 truncate text-[12px] font-medium text-(--text-muted)">
          {summary}
        </div>
        <div className="text-(--icon-muted)">
          {isExpanded ? (
            <ChevronDown className="h-3.5 w-3.5" />
          ) : (
            <ChevronRight className="h-3.5 w-3.5" />
          )}
        </div>
      </button>

      {!isExpanded ? (
        <WorkspaceFileArtifactList
          artifacts={collapsedFileArtifacts}
          className="ml-5 pb-1"
          label="生成文件"
          onOpenWorkspaceFile={onOpenWorkspaceFile}
        />
      ) : null}

      {isExpanded ? (
        <div className="pt-1">
          <ContentRenderer
            canRespondToPermissions={canRespondToPermissions}
            className="ml-1"
            content={processProjection.content}
            fallbackActivityState={fallbackActivityState}
            hiddenToolNames={hiddenToolNames}
            isStreaming={isStreaming}
            onOpenWorkspaceFile={onOpenWorkspaceFile}
            onPermissionResponse={onPermissionResponse}
            pendingPermissionsByToolUseId={pendingPermissionsByToolUseId}
            permissionReadOnlyReason={permissionReadOnlyReason}
            showTimelineDots
            streamingBlockIndexes={processProjection.streamingIndexes}
            workspaceAgentId={workspaceAgentId}
          />
          {pendingPermissionBlock}
        </div>
      ) : null}
    </div>
  );
}
