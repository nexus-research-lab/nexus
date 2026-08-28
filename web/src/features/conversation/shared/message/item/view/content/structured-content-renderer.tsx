"use client";

import { cn } from "@/shared/ui/class-name";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import { isSubagentToolName } from "../../../message-tool-names";

import { resolveContentActivityState } from "../../activity/message-content-activity";
import type { MessageActivityState } from "../../activity/message-activity-state";
import { LocalizedMessageActivityStatus } from "../message-activity-status";
import {
  ContentBlockView,
  type ContentBlockRenderContext,
} from "./content-block-view";
import type { StructuredContentRendererProps } from "./content-renderer-contract";
import {
  projectStructuredContent,
  type StructuredContentProjection,
} from "./content-renderer-model";
import { TIMELINE_LINE_CLASS_NAME } from "./content-renderer-timeline";

const EMPTY_HIDDEN_TOOL_NAMES: readonly string[] = [];
const NON_RENDERING_CONTENT_BLOCK_TYPES = new Set<ContentBlock["type"]>([
  "document",
  "progress_update",
  "redacted_thinking",
  "resource_link",
  "search_result",
  "task_progress",
  "tool_result",
  "unsupported",
]);

export function StructuredContentRenderer(
  props: StructuredContentRendererProps,
) {
  const {
    canRespondToPermissions,
    className,
    content,
    fallbackActivityLabel,
    fallbackActivityState,
    hiddenToolNames,
    isStreaming,
    onOpenSubagentTask,
    onOpenWorkspaceFile,
    onPermissionResponse,
    pendingInteractionOwner,
    pendingPermissionsByToolUseId,
    permissionReadOnlyReason,
    showTrailingActivity,
    showTimelineDots,
    streamingBlockIndexes,
    unresolvedToolStatus,
    workspaceAgentId,
    agentMentions,
    agentMentionDirectory,
    onOpenAgentContact,
  } = normalizeStructuredContentRendererProps(props);
  const projection = projectStructuredContent(content);
  const hiddenToolNameSet = new Set(hiddenToolNames);
  const activityState = resolveStructuredActivityState({
    content,
    fallbackActivityState,
    hiddenToolNames: hiddenToolNameSet,
    isStreaming,
    projection,
    streamingBlockIndexes,
  });
  const renderContext: ContentBlockRenderContext = {
    canRespondToPermissions,
    hiddenToolNames: hiddenToolNameSet,
    onOpenSubagentTask,
    onOpenWorkspaceFile,
    onPermissionResponse,
    pendingInteractionOwner,
    pendingPermissionsByToolUseId,
    permissionReadOnlyReason,
    projection,
    unresolvedToolStatus,
    workspaceAgentId,
    agentMentions,
    agentMentionDirectory,
    onOpenAgentContact,
  };
  const renderGroups = buildStructuredContentRenderGroups({
    content,
    enableSubagentTaskGroup: Boolean(onOpenSubagentTask) && !showTimelineDots,
    hiddenToolNames: hiddenToolNameSet,
    pendingPermissionsByToolUseId,
    projection,
  });

  return (
    <div
      className={cn(
        "nexus-chat-block-stack min-w-0 space-y-2.5",
        className,
        showTimelineDots && TIMELINE_LINE_CLASS_NAME,
      )}
    >
      {renderGroups.map((group) => (
        group.kind === "subagent_tasks" ? (
          <div
            className="flex min-w-0 flex-wrap gap-1.5 [&>div]:max-w-full"
            data-subagent-task-tool-group
            key={`subagent-tasks:${group.blockIndexes[0]}`}
          >
            {group.blockIndexes.map((blockIndex) => (
              <StructuredContentBlock
                block={content[blockIndex]}
                blockIndex={blockIndex}
                key={blockIndex}
                renderContext={renderContext}
                showTimelineDots={false}
                streaming={streamingBlockIndexes?.has(blockIndex) ?? false}
              />
            ))}
          </div>
        ) : (
          <StructuredContentBlock
            block={content[group.blockIndex]}
            blockIndex={group.blockIndex}
            key={group.blockIndex}
            renderContext={renderContext}
            showTimelineDots={showTimelineDots}
            streaming={streamingBlockIndexes?.has(group.blockIndex) ?? false}
          />
        )
      ))}
      {showTrailingActivity && activityState ? (
        <LocalizedMessageActivityStatus
          className="pt-1"
          label={fallbackActivityLabel}
          state={activityState}
        />
      ) : null}
    </div>
  );
}

type StructuredContentRenderGroup =
  | { blockIndex: number; kind: "block" }
  | { blockIndexes: number[]; kind: "subagent_tasks" };

function buildStructuredContentRenderGroups({
  content,
  enableSubagentTaskGroup,
  hiddenToolNames,
  pendingPermissionsByToolUseId,
  projection,
}: {
  content: readonly ContentBlock[];
  enableSubagentTaskGroup: boolean;
  hiddenToolNames: ReadonlySet<string>;
  pendingPermissionsByToolUseId?: ReadonlyMap<string, PendingPermission>;
  projection: StructuredContentProjection;
}): StructuredContentRenderGroup[] {
  const groups: StructuredContentRenderGroup[] = [];
  for (const [blockIndex, block] of content.entries()) {
    if (
      projection.consumedBlockIndexes.has(blockIndex)
      || NON_RENDERING_CONTENT_BLOCK_TYPES.has(block.type)
    ) {
      continue;
    }
    const isSubagentTask = enableSubagentTaskGroup
      && block.type === "tool_use"
      && !hiddenToolNames.has(block.name)
      && !pendingPermissionsByToolUseId?.has(block.id)
      && isSubagentToolName(block.name);
    const previousGroup = groups.at(-1);
    if (isSubagentTask && previousGroup?.kind === "subagent_tasks") {
      previousGroup.blockIndexes.push(blockIndex);
      continue;
    }
    groups.push(isSubagentTask
      ? { blockIndexes: [blockIndex], kind: "subagent_tasks" }
      : { blockIndex, kind: "block" });
  }
  return groups;
}

function normalizeStructuredContentRendererProps(
  props: StructuredContentRendererProps,
) {
  return {
    ...props,
    canRespondToPermissions: props.canRespondToPermissions ?? true,
    fallbackActivityLabel: props.fallbackActivityLabel ?? null,
    fallbackActivityState: props.fallbackActivityState ?? null,
    hiddenToolNames: props.hiddenToolNames ?? EMPTY_HIDDEN_TOOL_NAMES,
    isStreaming: props.isStreaming ?? false,
    pendingInteractionOwner: props.pendingInteractionOwner ?? "content",
    showTrailingActivity: props.showTrailingActivity ?? true,
    showTimelineDots: props.showTimelineDots ?? false,
  };
}

function StructuredContentBlock({
  block,
  blockIndex,
  renderContext,
  showTimelineDots,
  streaming,
}: {
  block: ContentBlock;
  blockIndex: number;
  renderContext: ContentBlockRenderContext;
  showTimelineDots: boolean;
  streaming: boolean;
}) {
  return (
    <ContentBlockView
      block={block}
      context={renderContext}
      showTimelineDots={showTimelineDots}
      streaming={streaming}
      blockIndex={blockIndex}
    />
  );
}

function resolveStructuredActivityState({
  content,
  fallbackActivityState,
  hiddenToolNames,
  isStreaming,
  projection,
  streamingBlockIndexes,
}: {
  content: readonly ContentBlock[];
  fallbackActivityState: MessageActivityState | null;
  hiddenToolNames: ReadonlySet<string>;
  isStreaming: boolean;
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
    resolvedToolUseIds: projection.resolvedToolUseIds,
    streamingBlockIndexes,
  });
}
