"use client";

import { useMemo } from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import {
  getSystemMessageDisplayMeta,
  type AssistantMessage,
  type Message,
  type SystemEventContent,
} from "@/types/conversation/message";
import type { PendingPermission } from "@/types/conversation/permission";

import type { AssistantContentMode, ContentProjection } from "../message-item-support";
import { buildProcessSummary, resolveLiveActivityState } from "./message-item-activity";
import { resolveMessageItemFinalProjection } from "./message-item-final-projection";
import {
  buildVisibleAssistantTurns,
  buildVisibleOrderedAssistantEntries,
} from "./message-item-ordering";
import { resolveMessageItemPermissions } from "./message-item-permissions";
import { buildMessageStats } from "./message-item-stats";

interface MessageItemProjectionOptions {
  assistantContentMode: AssistantContentMode;
  hiddenToolNames: string[];
  isLastRound?: boolean;
  isLoading?: boolean;
  messages: Message[];
  pendingPermissions: PendingPermission[];
  roundId: string;
  runtimePhase?: AgentConversationRuntimePhase | null;
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
  const contentMerge = useAssistantContentMerge({
    messages,
    isLastRound,
    isLoading,
  });
  const firstAssistant = contentMerge.assistantMessages[0] as AssistantMessage | undefined;
  const streamStatus = firstAssistant?.stream_status ?? null;
  const systemEventBlocks = useMemo(
    () => buildSystemEventBlocks(messages, Boolean(isLastRound && isLoading)),
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
  const hiddenToolUseIds = useMemo(() => {
    const ids = new Set<string>();
    for (const block of contentMerge.mergedContent) {
      if (block.type === "tool_use" && hiddenToolNameSet.has(block.name)) {
        ids.add(block.id);
      }
    }
    return ids;
  }, [contentMerge.mergedContent, hiddenToolNameSet]);

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
  const orderedProjection = useMemo<ContentProjection>(() => {
    const streamingIndexes = new Set<number>();
    visibleOrderedAssistantEntries.forEach((entry, visibleIndex) => {
      if (contentMerge.streamingBlockIndexes.has(entry.mergedIndex)) {
        streamingIndexes.add(visibleIndex);
      }
    });
    return {
      content: visibleOrderedAssistantEntries.map((entry) => entry.block),
      streamingIndexes,
    };
  }, [contentMerge.streamingBlockIndexes, visibleOrderedAssistantEntries]);
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
  const finalProjection = useMemo(
    () => resolveMessageItemFinalProjection({
      assistantContentMode,
      assistantMessages: contentMerge.assistantMessages,
      orderedProjection,
      resultSummary: contentMerge.resultSummary,
      roundId,
      userMessageId: contentMerge.userMessage?.message_id ?? null,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
      visibleAssistantTurns,
      visibleOrderedAssistantEntries,
    }),
    [
      assistantContentMode,
      contentMerge.assistantMessages,
      contentMerge.resultSummary,
      contentMerge.streamingBlockIndexes,
      contentMerge.userMessage,
      orderedProjection,
      roundId,
      visibleAssistantTurns,
      visibleOrderedAssistantEntries,
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
      streamStatus,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
    }),
    [
      contentMerge.mergedContent,
      contentMerge.streamingBlockIndexes,
      isLastRound,
      isLoading,
      pendingPermissions,
      runtimePhase,
      streamStatus,
    ],
  );
  const processSummary = useMemo(
    () => buildProcessSummary({
      pendingPermissionCount: pendingPermissions.length,
      processContent: finalProjection.processProjection.content,
    }),
    [pendingPermissions.length, finalProjection.processProjection.content],
  );

  const userMessage = contentMerge.userMessage;
  const userContent =
    userMessage?.role === "user" && typeof userMessage.content === "string"
      ? userMessage.content
      : "";

  return {
    assistantMessages: contentMerge.assistantMessages,
    mergedContent: contentMerge.mergedContent,
    resultSummary: contentMerge.resultSummary,
    streamingBlockIndexes: contentMerge.streamingBlockIndexes,
    userMessage,
    ...finalProjection,
    ...permissionMatch,
    assistantAgentId: firstAssistant?.agent_id ?? null,
    firstAssistantMessageId: firstAssistant?.message_id ?? null,
    liveActivityState,
    model: firstAssistant?.model,
    processSummary,
    stats: buildMessageStats(contentMerge.resultSummary),
    stopReason: firstAssistant?.stop_reason ?? null,
    streamStatus,
    timestamp:
      firstAssistant?.timestamp ??
      systemEventBlocks[0]?.timestamp ??
      contentMerge.resultSummary?.timestamp,
    userAttachments: userMessage?.role === "user" ? userMessage.attachments ?? [] : [],
    userContent,
  };
}

function buildSystemEventBlocks(
  messages: Message[],
  includeTransientEvents: boolean,
): SystemEventContent[] {
  return messages.flatMap((message) => {
    if (
      message.role !== "system" ||
      typeof message.content !== "string" ||
      !message.content.trim() ||
      (!includeTransientEvents && message.metadata?.subtype !== "guided_input")
    ) {
      return [];
    }

    const displayMeta = getSystemMessageDisplayMeta(message);
    return [{
      type: "system_event" as const,
      content: message.content,
      label: displayMeta.label,
      tone: displayMeta.tone,
      icon: displayMeta.icon,
      source_message_id: message.message_id,
      timestamp: message.timestamp,
      subtype: message.metadata?.subtype,
      tool_use_id:
        typeof message.metadata?.tool_use_id === "string"
          ? message.metadata.tool_use_id
          : null,
    }];
  });
}
