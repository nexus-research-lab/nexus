import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import {
  type MessageActivityState,
  resolvePermissionActivityState,
} from "./message-activity-state";
import { findLastActivityBlock } from "./message-activity-blocks";

const RUNTIME_PHASE_ACTIVITY: Partial<
  Record<AgentConversationRuntimePhase, MessageActivityState>
> = {
  awaiting_permission: "waiting_permission",
  running: "thinking",
  sending: "sending",
  streaming: "replying",
};

const STREAMING_BLOCK_ACTIVITY: Partial<
  Record<ContentBlock["type"], MessageActivityState>
> = {
  text: "replying",
  thinking: "thinking",
};

const STREAMING_TEXT_READERS: ReadonlyArray<
  (block: ContentBlock) => string | null
> = [
  (block) => block.type === "text" ? block.text : null,
  (block) => block.type === "thinking" ? block.thinking : null,
  (block) => block.type === "tool_use_error" ? block.content : null,
];

interface LiveActivityContext {
  hasVisibleReplyText: boolean;
  permissionActivity: MessageActivityState | null;
  runtimeActivity: MessageActivityState | null;
  streamingActivity: MessageActivityState | null;
  streamStatus?: string | null;
}

const LIVE_ACTIVITY_RESOLVERS: ReadonlyArray<
  (context: LiveActivityContext) => MessageActivityState | null
> = [
  ({ permissionActivity }) => permissionActivity,
  ({ runtimeActivity }) => runtimeActivity === "sending" ? "sending" : null,
  ({ streamingActivity }) => streamingActivity,
  ({ hasVisibleReplyText, streamStatus }) => (
    hasVisibleReplyText && streamStatus === "streaming" ? "replying" : null
  ),
  ({ streamStatus }) => streamStatus === "pending" ? "thinking" : null,
  ({ runtimeActivity }) => runtimeActivity,
];

export function resolveLiveActivityState({
  isLastRound,
  isLoading,
  mergedContent,
  pendingPermissions,
  runtimePhase,
  streamStatus,
  streamingBlockIndexes,
}: {
  isLastRound?: boolean;
  isLoading?: boolean;
  mergedContent: readonly ContentBlock[];
  pendingPermissions: readonly PendingPermission[];
  runtimePhase?: AgentConversationRuntimePhase | null;
  streamStatus?: string | null;
  streamingBlockIndexes: ReadonlySet<number>;
}): MessageActivityState | null {
  if (!isLastRound || !isLoading) {
    return null;
  }

  const context = buildLiveActivityContext({
    mergedContent,
    pendingPermissions,
    runtimePhase,
    streamStatus,
    streamingBlockIndexes,
  });
  return LIVE_ACTIVITY_RESOLVERS
    .map((resolveActivity) => resolveActivity(context))
    .find((candidate) => candidate !== null) ?? null;
}

function buildLiveActivityContext({
  mergedContent,
  pendingPermissions,
  runtimePhase,
  streamStatus,
  streamingBlockIndexes,
}: {
  mergedContent: readonly ContentBlock[];
  pendingPermissions: readonly PendingPermission[];
  runtimePhase?: AgentConversationRuntimePhase | null;
  streamStatus?: string | null;
  streamingBlockIndexes: ReadonlySet<number>;
}): LiveActivityContext {
  return {
    hasVisibleReplyText: mergedContent.some(
      (block) => block.type === "text" && Boolean(block.text.trim()),
    ),
    permissionActivity: resolvePermissionActivityState(pendingPermissions),
    runtimeActivity: resolveRuntimeActivityState(runtimePhase),
    streamingActivity: resolveStreamingBlockActivity(
      mergedContent,
      streamingBlockIndexes,
    ),
    streamStatus,
  };
}

function resolveRuntimeActivityState(
  phase?: AgentConversationRuntimePhase | null,
): MessageActivityState | null {
  return phase ? RUNTIME_PHASE_ACTIVITY[phase] ?? null : null;
}

function resolveStreamingBlockActivity(
  content: readonly ContentBlock[],
  streamingBlockIndexes: ReadonlySet<number>,
): MessageActivityState | null {
  const block = findLatestStreamingBlock(content, streamingBlockIndexes);
  return block ? STREAMING_BLOCK_ACTIVITY[block.type] ?? null : null;
}

function findLatestStreamingBlock(
  content: readonly ContentBlock[],
  streamingBlockIndexes: ReadonlySet<number>,
): ContentBlock | null {
  return findLastActivityBlock(
    content,
    (block, index): block is ContentBlock => streamingBlockIndexes.has(index)
      && !isEmptyStreamingBlock(block),
  );
}

function isEmptyStreamingBlock(block: ContentBlock): boolean {
  const text = STREAMING_TEXT_READERS
    .map((readText) => readText(block))
    .find((value) => value !== null);
  return text === undefined ? false : !text.trim();
}
