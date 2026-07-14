/**
 * INPUT: 一个 root round 的消息、权限与 runtime 状态。
 * OUTPUT: MessageItem 所需的 user 列表、assistant 内容与展示状态。
 * POS: 会话消息实体到单轮视图模型的投影边界。
 */
import { useMemo } from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type { ContentBlock } from "@/types/conversation/message/content";
import type {
  AssistantMessage,
  Message,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { resolveLiveActivityState } from "../../activity/message-live-activity";
import {
  projectionFromOrderedEntries,
  type AssistantContentMode,
} from "../../message-item-projection";
import { buildProcessSummary } from "../../process/message-process-summary";
import { resolveMessageItemFinalProjection } from "./message-item-final-projection";
import {
  buildVisibleAssistantTurns,
  buildVisibleOrderedAssistantEntries,
} from "./message-item-ordering";
import { resolveMessageItemPermissions } from "./message-item-permissions";
import { buildMessageStats } from "./message-item-stats";
import { buildSystemEventBlocks } from "./message-item-system-events";

interface MessageItemProjectionOptions {
  assistantContentMode: AssistantContentMode;
  hiddenToolNames: readonly string[];
  isLastRound?: boolean;
  isLoading?: boolean;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  roundId: string;
  runtimePhase?: AgentConversationRuntimePhase | null;
}

interface OrderedContentProjectionOptions {
  hiddenToolNames: readonly string[];
  isLastRound?: boolean;
  isLoading?: boolean;
  messages: Message[];
}

interface AssistantIdentityProjection {
  assistantAgentId: string | null;
  firstAssistantMessageId: string | null;
  model: string | undefined;
  stopReason: string | null;
  streamStatus: string | null;
}

export function useMessageItemProjection({
  assistantContentMode,
  hiddenToolNames,
  isLastRound,
  isLoading,
  messages,
  pendingPermissions,
  roundId,
  runtimePhase,
}: MessageItemProjectionOptions) {
  const orderedContent = useOrderedContentProjection({
    hiddenToolNames,
    isLastRound,
    isLoading,
    messages,
  });
  const { contentMerge } = orderedContent;
  const firstAssistant = contentMerge.assistantMessages[0];
  const assistantIdentity = projectAssistantIdentity(firstAssistant);
  const finalProjection = useMemo(
    () => resolveMessageItemFinalProjection({
      assistantContentMode,
      assistantMessages: contentMerge.assistantMessages,
      orderedProjection: orderedContent.orderedProjection,
      resultSummary: contentMerge.resultSummary,
      roundId,
      userMessageId: contentMerge.userMessages.at(0)?.message_id ?? null,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
      visibleAssistantTurns: orderedContent.visibleAssistantTurns,
      visibleOrderedAssistantEntries:
        orderedContent.visibleOrderedAssistantEntries,
    }),
    [
      assistantContentMode,
      contentMerge.assistantMessages,
      contentMerge.resultSummary,
      contentMerge.streamingBlockIndexes,
      contentMerge.userMessages,
      orderedContent.orderedProjection,
      orderedContent.visibleAssistantTurns,
      orderedContent.visibleOrderedAssistantEntries,
      roundId,
    ],
  );
  const permissionMatch = useMemo(
    () => resolveMessageItemPermissions(messages, pendingPermissions),
    [messages, pendingPermissions],
  );
  const liveActivityState = useMemo(
    () => resolveLiveActivityState({
      isLastRound,
      isLoading,
      mergedContent: contentMerge.mergedContent,
      pendingPermissions,
      runtimePhase,
      streamStatus: assistantIdentity.streamStatus,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
    }),
    [
      assistantIdentity.streamStatus,
      contentMerge.mergedContent,
      contentMerge.streamingBlockIndexes,
      isLastRound,
      isLoading,
      pendingPermissions,
      runtimePhase,
    ],
  );
  const processSummary = useMemo(
    () => buildProcessSummary({
      pendingPermissionCount: pendingPermissions.length,
      processContent: finalProjection.processProjection.content,
    }),
    [pendingPermissions.length, finalProjection.processProjection.content],
  );
  return {
    assistantMessages: contentMerge.assistantMessages,
    mergedContent: contentMerge.mergedContent,
    resultSummary: contentMerge.resultSummary,
    streamingBlockIndexes: contentMerge.streamingBlockIndexes,
    ...assistantIdentity,
    ...finalProjection,
    ...permissionMatch,
    userMessages: contentMerge.userMessages,
    liveActivityState,
    processSummary,
    stats: buildMessageStats(contentMerge.resultSummary),
    timestamp: resolveMessageTimestamp(
      firstAssistant,
      orderedContent.firstSystemEventTimestamp,
      contentMerge.resultSummary,
    ),
  };
}

function useOrderedContentProjection({
  hiddenToolNames,
  isLastRound,
  isLoading,
  messages,
}: OrderedContentProjectionOptions) {
  const contentMerge = useAssistantContentMerge({
    isLastRound,
    isLoading,
    messages,
  });
  const systemEventBlocks = useMemo(
    () => buildSystemEventBlocks(
      messages,
      shouldIncludeTransientSystemEvents(isLastRound, isLoading),
    ),
    [isLastRound, isLoading, messages],
  );
  const sourceMessageOrderById = useMemo(
    () => new Map(messages.map((message, index) => [message.message_id, index])),
    [messages],
  );
  const hiddenToolNameSet = useMemo(
    () => new Set(hiddenToolNames),
    [hiddenToolNames],
  );
  const hiddenToolUseIds = useMemo(
    () => collectHiddenToolUseIds(
      contentMerge.mergedContent,
      hiddenToolNameSet,
    ),
    [contentMerge.mergedContent, hiddenToolNameSet],
  );
  const visibleOrderedAssistantEntries = useMemo(
    () => buildVisibleOrderedAssistantEntries({
      hiddenToolNames: hiddenToolNameSet,
      hiddenToolUseIds,
      isLoading,
      mergedContent: contentMerge.mergedContent,
      mergedContentSourceMessageIds: contentMerge.mergedContentSourceMessageIds,
      sourceMessageOrderById,
      systemEventBlocks,
    }),
    [
      contentMerge.mergedContent,
      contentMerge.mergedContentSourceMessageIds,
      hiddenToolNameSet,
      hiddenToolUseIds,
      isLoading,
      sourceMessageOrderById,
      systemEventBlocks,
    ],
  );
  const orderedProjection = useMemo(
    () => projectionFromOrderedEntries(
      visibleOrderedAssistantEntries,
      contentMerge.streamingBlockIndexes,
    ),
    [contentMerge.streamingBlockIndexes, visibleOrderedAssistantEntries],
  );
  const visibleAssistantTurns = useMemo(
    () => buildVisibleAssistantTurns({
      assistantMessages: contentMerge.assistantMessages,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
      visibleOrderedAssistantEntries,
    }),
    [
      contentMerge.assistantMessages,
      contentMerge.streamingBlockIndexes,
      visibleOrderedAssistantEntries,
    ],
  );

  return {
    contentMerge,
    firstSystemEventTimestamp: systemEventBlocks.at(0)?.timestamp,
    orderedProjection,
    visibleAssistantTurns,
    visibleOrderedAssistantEntries,
  };
}

function collectHiddenToolUseIds(
  content: readonly ContentBlock[],
  hiddenToolNames: ReadonlySet<string>,
): Set<string> {
  const ids = new Set<string>();
  for (const block of content) {
    if (block.type === "tool_use" && hiddenToolNames.has(block.name)) {
      ids.add(block.id);
    }
  }
  return ids;
}

function shouldIncludeTransientSystemEvents(
  isLastRound: boolean | undefined,
  isLoading: boolean | undefined,
): boolean {
  return [isLastRound, isLoading].every(Boolean);
}

function projectAssistantIdentity(
  firstAssistant: AssistantMessage | undefined,
): AssistantIdentityProjection {
  if (!firstAssistant) {
    return {
      assistantAgentId: null,
      firstAssistantMessageId: null,
      model: undefined,
      stopReason: null,
      streamStatus: null,
    };
  }
  return {
    assistantAgentId: firstAssistant.agent_id ?? null,
    firstAssistantMessageId: firstAssistant.message_id,
    model: firstAssistant.model,
    stopReason: firstAssistant.stop_reason ?? null,
    streamStatus: firstAssistant.stream_status ?? null,
  };
}

function resolveMessageTimestamp(
  firstAssistant: AssistantMessage | undefined,
  firstSystemEventTimestamp: number | undefined,
  resultSummary: ResultSummary | undefined,
): number | undefined {
  return firstAssistant?.timestamp
    ?? firstSystemEventTimestamp
    ?? resultSummary?.timestamp;
}
