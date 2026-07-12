"use client";

import { cn } from "@/shared/ui/class-name";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { resolveContentActivityState } from "../../activity/message-content-activity";
import type { MessageActivityState } from "../../activity/message-activity-state";
import { MessageActivityStatus } from "../message-activity-status";
import {
  ContentBlockView,
  type ContentBlockRenderContext,
} from "./content-block-view";
import type { StructuredContentRendererProps } from "./content-renderer-contract";
import {
  projectStructuredContent,
  type StructuredContentProjection,
} from "./content-renderer-model";
import {
  TIMELINE_LINE_CLASS_NAME,
} from "./content-renderer-timeline";

const EMPTY_HIDDEN_TOOL_NAMES: readonly string[] = [];

export function StructuredContentRenderer(
  props: StructuredContentRendererProps,
) {
  const {
    canRespondToPermissions,
    className,
    content,
    fallbackActivityState,
    hiddenToolNames,
    isStreaming,
    onOpenWorkspaceFile,
    onPermissionResponse,
    pendingPermissionsByToolUseId,
    permissionReadOnlyReason,
    showTimelineDots,
    streamingBlockIndexes,
    workspaceAgentId,
  } = normalizeStructuredContentRendererProps(props);
  const projection = projectStructuredContent(content);
  const hiddenToolNameSet = new Set(hiddenToolNames);
  const activityState = resolveStructuredActivityState({
    content,
    fallbackActivityState,
    hiddenToolNames: hiddenToolNameSet,
    isStreaming,
    pendingPermissionsByToolUseId,
    projection,
    streamingBlockIndexes,
  });
  const renderContext: ContentBlockRenderContext = {
    canRespondToPermissions,
    hiddenToolNames: hiddenToolNameSet,
    onOpenWorkspaceFile,
    onPermissionResponse,
    pendingPermissionsByToolUseId,
    permissionReadOnlyReason,
    projection,
    workspaceAgentId,
  };

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-2.5",
        className,
        showTimelineDots && TIMELINE_LINE_CLASS_NAME,
      )}
    >
      {content.map((block, index) => (
        <StructuredContentBlock
          block={block}
          consumed={projection.consumedBlockIndexes.has(index)}
          key={index}
          renderContext={renderContext}
          showTimelineDots={showTimelineDots}
          streaming={streamingBlockIndexes?.has(index) ?? false}
        />
      ))}
      {activityState ? (
        <MessageActivityStatus className="pt-1" state={activityState} />
      ) : null}
    </div>
  );
}

function normalizeStructuredContentRendererProps(
  props: StructuredContentRendererProps,
) {
  return {
    ...props,
    canRespondToPermissions: props.canRespondToPermissions ?? true,
    fallbackActivityState: props.fallbackActivityState ?? null,
    hiddenToolNames: props.hiddenToolNames ?? EMPTY_HIDDEN_TOOL_NAMES,
    isStreaming: props.isStreaming ?? false,
    showTimelineDots: props.showTimelineDots ?? false,
  };
}

function StructuredContentBlock({
  block,
  consumed,
  renderContext,
  showTimelineDots,
  streaming,
}: {
  block: ContentBlock;
  consumed: boolean;
  renderContext: ContentBlockRenderContext;
  showTimelineDots: boolean;
  streaming: boolean;
}) {
  if (consumed) {
    return null;
  }
  return (
    <ContentBlockView
      block={block}
      context={renderContext}
      showTimelineDots={showTimelineDots}
      streaming={streaming}
    />
  );
}

function resolveStructuredActivityState({
  content,
  fallbackActivityState,
  hiddenToolNames,
  isStreaming,
  pendingPermissionsByToolUseId,
  projection,
  streamingBlockIndexes,
}: {
  content: readonly ContentBlock[];
  fallbackActivityState: MessageActivityState | null;
  hiddenToolNames: ReadonlySet<string>;
  isStreaming: boolean;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  projection: StructuredContentProjection;
  streamingBlockIndexes?: ReadonlySet<number>;
}): MessageActivityState | null {
  if (!isStreaming) {
    return null;
  }
  return resolveContentActivityState({
    consumedBlockIndexes: projection.consumedBlockIndexes,
    content,
    fallbackActivityState,
    hiddenToolNames,
    pendingPermissionsByToolUseId,
    resolvedToolUseIds: projection.resolvedToolUseIds,
    streamingBlockIndexes,
  });
}
