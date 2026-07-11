import type { ReactNode } from "react";

import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { ImageBlock } from "../../../blocks/artifact/image/image-block";
import { WorkspaceFileArtifactBlock } from "../../../blocks/artifact/workspace-file-artifacts";
import { ThinkingBlock } from "../../../blocks/thinking-block";
import { ToolUseErrorBlock } from "../../../blocks/tool/tool-use-error-block";
import { MarkdownRenderer } from "../../../markdown-renderer";
import {
  isHiddenSystemEvent,
  type StructuredContentProjection,
} from "./content-renderer-model";
import { ContentSystemEvent } from "./content-system-event";
import { renderContentToolBlock } from "./content-tool-block";

interface ContentBlockViewOptions {
  block: ContentBlock;
  blockIsStreaming: boolean;
  canRespondToPermissions: boolean;
  hiddenToolNames: ReadonlySet<string>;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  projection: StructuredContentProjection;
  workspaceAgentId?: string | null;
}

export function renderContentBlock({
  block,
  blockIsStreaming,
  canRespondToPermissions,
  hiddenToolNames,
  onOpenWorkspaceFile,
  onPermissionResponse,
  pendingPermissionsByToolUseId,
  permissionReadOnlyReason,
  projection,
  workspaceAgentId,
}: ContentBlockViewOptions): ReactNode {
  switch (block.type) {
    case "text":
      return block.text.trim() ? (
        <MarkdownRenderer
          content={block.text}
          isStreaming={blockIsStreaming}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      ) : null;
    case "tool_use_error":
      return <ToolUseErrorBlock content={block.content} />;
    case "thinking":
      return block.thinking.trim() ? (
        <ThinkingBlock
          isStreaming={blockIsStreaming}
          thinking={block.thinking}
          workspaceAgentId={workspaceAgentId}
        />
      ) : null;
    case "image":
      return (
        <ImageBlock
          block={block}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
          workspaceAgentId={workspaceAgentId}
        />
      );
    case "system_event":
      return isHiddenSystemEvent(block) ? null : <ContentSystemEvent block={block} />;
    case "workspace_file_artifact":
      return (
        <WorkspaceFileArtifactBlock
          artifact={block}
          onOpenWorkspaceFile={onOpenWorkspaceFile}
        />
      );
    case "tool_use":
      return hiddenToolNames.has(block.name) ? null : renderContentToolBlock({
        block,
        canRespondToPermissions,
        onOpenWorkspaceFile,
        onPermissionResponse,
        pendingPermission: pendingPermissionsByToolUseId?.get(block.id),
        permissionReadOnlyReason,
        projection,
        workspaceAgentId,
      });
    case "task_progress":
    case "tool_result":
      return null;
  }
}
