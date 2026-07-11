"use client";

import type { Key, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

import { resolveContentActivityState } from "../../activity/message-content-activity";
import type { MessageActivityState } from "../../activity/message-activity-state";
import { MarkdownRenderer } from "../../../markdown-renderer";
import { MessageActivityStatus } from "../message-activity-status";
import { renderContentBlock } from "./content-block-view";
import { projectStructuredContent } from "./content-renderer-model";
import { TimelineBlock } from "./content-renderer-timeline";

const EMPTY_HIDDEN_TOOL_NAMES: string[] = [];
const TIMELINE_LINE_CLASS_NAME =
  "relative before:absolute before:bottom-0 before:left-[5.5px] before:top-0 before:w-px before:bg-(--divider-subtle-color)";

interface ContentRendererProps {
  canRespondToPermissions?: boolean;
  className?: string;
  content: string | ContentBlock[];
  fallbackActivityState?: MessageActivityState | null;
  hiddenToolNames?: string[];
  isStreaming?: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  permissionReadOnlyReason?: string;
  showTimelineDots?: boolean;
  streamingBlockIndexes?: ReadonlySet<number>;
  workspaceAgentId?: string | null;
}

export function ContentRenderer({
  canRespondToPermissions = true,
  className,
  content,
  fallbackActivityState,
  hiddenToolNames = EMPTY_HIDDEN_TOOL_NAMES,
  isStreaming = false,
  onOpenWorkspaceFile,
  onPermissionResponse,
  pendingPermissionsByToolUseId,
  permissionReadOnlyReason,
  showTimelineDots = false,
  streamingBlockIndexes,
  workspaceAgentId,
}: ContentRendererProps) {
  if (typeof content === "string") {
    return (
      <MarkdownContent
        className={className}
        content={content}
        isStreaming={isStreaming}
        onOpenWorkspaceFile={onOpenWorkspaceFile}
        showTimelineDots={showTimelineDots}
        workspaceAgentId={workspaceAgentId}
      />
    );
  }

  const projection = projectStructuredContent(content);
  const hiddenToolNameSet = new Set(hiddenToolNames);
  const activityState = isStreaming
    ? resolveContentActivityState({
      consumedBlockIndexes: projection.consumedBlockIndexes,
      content,
      fallbackActivityState,
      hiddenToolNames: hiddenToolNameSet,
      pendingPermissionsByToolUseId,
      resolvedToolUseIds: projection.resolvedToolUseIds,
      streamingBlockIndexes,
    })
    : null;

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-2.5",
        className,
        showTimelineDots && TIMELINE_LINE_CLASS_NAME,
      )}
    >
      {content.map((block, index) => {
        if (projection.consumedBlockIndexes.has(index)) {
          return null;
        }
        const blockIsStreaming = streamingBlockIndexes?.has(index) ?? false;
        const node = renderContentBlock({
          block,
          blockIsStreaming,
          canRespondToPermissions,
          hiddenToolNames: hiddenToolNameSet,
          onOpenWorkspaceFile,
          onPermissionResponse,
          pendingPermissionsByToolUseId,
          permissionReadOnlyReason,
          projection,
          workspaceAgentId,
        });
        return wrapContentBlock(index, node, showTimelineDots, blockIsStreaming);
      })}
      {activityState ? (
        <MessageActivityStatus className="pt-1" state={activityState} />
      ) : null}
    </div>
  );
}

function MarkdownContent({
  className,
  content,
  isStreaming,
  onOpenWorkspaceFile,
  showTimelineDots,
  workspaceAgentId,
}: {
  className?: string;
  content: string;
  isStreaming: boolean;
  onOpenWorkspaceFile?: (path: string) => void;
  showTimelineDots: boolean;
  workspaceAgentId?: string | null;
}) {
  const markdown = (
    <MarkdownRenderer
      content={content}
      isStreaming={isStreaming}
      onOpenWorkspaceFile={onOpenWorkspaceFile}
      workspaceAgentId={workspaceAgentId}
    />
  );
  if (!className) {
    return markdown;
  }

  return (
    <div className={cn(className, showTimelineDots && TIMELINE_LINE_CLASS_NAME)}>
      {showTimelineDots ? (
        <TimelineBlock active={isStreaming}>{markdown}</TimelineBlock>
      ) : markdown}
    </div>
  );
}

function wrapContentBlock(
  key: Key,
  node: ReactNode,
  showTimelineDots: boolean,
  active: boolean,
) {
  if (node === null || node === undefined || node === false) {
    return null;
  }
  if (!showTimelineDots) {
    return <div key={key}>{node}</div>;
  }
  return (
    <TimelineBlock active={active} key={key}>
      {node}
    </TimelineBlock>
  );
}
