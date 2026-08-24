/**
 * INPUT: 一个 root round 的消息（含结构化来源 metadata）、权限与 runtime 状态。
 * OUTPUT: MessageItem 所需的 user/assistant 内容、排除 ephemeral 的身份、欢迎语模型隐藏和即时自然语言活动标签。
 * POS: 会话消息实体到单轮视图模型的投影边界。
 */
import { useMemo } from "react";

import { useAssistantContentMerge } from "@/hooks/conversation/use-assistant-content-merge";
import type { AgentConversationRuntimePhase } from "@/types/agent/agent-conversation";
import type {
  AssistantMessage,
  GoalCompletionReceipt,
  Message,
  RecalledMemoryReference,
  ResultSummary,
} from "@/types/conversation/message/entity";
import type { ContentBlock } from "@/types/conversation/message/content";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import { resolveLiveActivityState } from "../../activity/message-live-activity";
import type { MessageActivityState } from "../../activity/message-activity-state";
import {
  projectionFromOrderedEntries,
  type AssistantContentMode,
  type ToolUseSummaryProjection,
} from "../../message-item-projection";
import { buildProcessSummary } from "../../process/message-process-summary";
import { resolveMessageItemFinalProjection } from "./message-item-final-projection";
import {
  buildVisibleAssistantTurns,
  buildVisibleOrderedAssistantEntries,
  collectHiddenToolUseIds,
} from "./message-item-ordering";
import { resolveMessageItemPermissions } from "./message-item-permissions";
import { buildMessageStats } from "./message-item-stats";
import { buildSystemEventBlocks } from "./message-item-system-events";

interface MessageItemProjectionOptions {
  activityState?: MessageActivityState | null;
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
  automationTaskName: string | null;
  echo: boolean;
  firstAssistantMessageId: string | null;
  model: string | undefined;
  stopReason: string | null;
  streamStatus: string | null;
}

export function useMessageItemProjection({
  activityState,
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
  const identityAssistant = selectAssistantIdentity(
    contentMerge.assistantMessages,
  );
  const assistantIdentity = projectAssistantIdentity(
    contentMerge.assistantMessages,
    identityAssistant,
  );
  const goalCompletionReceipt = resolveGoalCompletionReceipt(
    contentMerge.assistantMessages,
  );
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
    () => resolveMessageItemPermissions(
      messages,
      pendingPermissions,
      collectVisibleToolUseIds(finalProjection),
    ),
    [finalProjection, messages, pendingPermissions],
  );
  const liveActivityState = useMemo(
    () => resolveLiveActivityState({
      activityState,
      isLastRound,
      isLoading,
      mergedContent: contentMerge.mergedContent,
      pendingPermissions,
      runtimePhase,
      streamStatus: assistantIdentity.streamStatus,
      streamingBlockIndexes: contentMerge.streamingBlockIndexes,
    }),
    [
      activityState,
      assistantIdentity.streamStatus,
      contentMerge.mergedContent,
      contentMerge.streamingBlockIndexes,
      isLastRound,
      isLoading,
      pendingPermissions,
      runtimePhase,
    ],
  );
  const liveToolUseSummary = useMemo(
    () => resolveActivityToolUseSummary(
      contentMerge.mergedContent,
      liveActivityState,
    ),
    [contentMerge.mergedContent, liveActivityState],
  );
  const liveActivityLabel = liveToolUseSummary?.text ?? null;
  const processSummary = useMemo(
    () => buildProcessSummary({
      pendingPermissionCount: pendingPermissions.length,
      processContent: finalProjection.processProjection.content,
    }),
    [pendingPermissions.length, finalProjection.processProjection.content],
  );
  const recalledMemories = useMemo(
    () => collectRecalledMemoryReferences(contentMerge.assistantMessages),
    [contentMerge.assistantMessages],
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
    liveActivityLabel,
    liveToolUseSummary,
    goalCompletionReceipt,
    processSummary,
    recalledMemories,
    stats: buildMessageStats(contentMerge.resultSummary),
    timestamp: resolveMessageTimestamp(
      identityAssistant,
      orderedContent.firstSystemEventTimestamp,
      contentMerge.resultSummary,
    ),
  };
}

const PROGRESS_LABEL_ACTIVITY_STATES = new Set<MessageActivityState>([
  "browsing",
  "executing",
  "replying",
  "thinking",
]);

export function resolveActivityProgressLabel(
  content: readonly ContentBlock[],
  activityState: MessageActivityState | null,
): string | null {
  return resolveActivityToolUseSummary(content, activityState)?.text ?? null;
}

export function resolveActivityToolUseSummary(
  content: readonly ContentBlock[],
  activityState: MessageActivityState | null,
): ToolUseSummaryProjection | null {
  if (!activityState || !PROGRESS_LABEL_ACTIVITY_STATES.has(activityState)) {
    return null;
  }
  for (let index = content.length - 1; index >= 0; index -= 1) {
    const block = content[index];
    if (block.type === "progress_update") {
      const text = block.text.trim();
      if (!text) {
        return null;
      }
      return {
        precedingToolUseIds: [...new Set(
          (block.preceding_tool_use_ids ?? [])
            .map((toolUseId) => toolUseId.trim())
            .filter(Boolean),
        )],
        text,
      };
    }
  }
  return null;
}

function resolveGoalCompletionReceipt(
  messages: readonly AssistantMessage[],
): GoalCompletionReceipt | null {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const receipt = messages[index]?.goal_completion_receipt;
    if (receipt?.goal_id?.trim() && receipt.round_id?.trim()) {
      return receipt;
    }
  }
  return null;
}

export function collectRecalledMemoryReferences(
  messages: readonly AssistantMessage[],
): RecalledMemoryReference[] {
  const seen = new Set<string>();
  const references: RecalledMemoryReference[] = [];
  for (const message of messages) {
    for (const reference of message.recalled_memories ?? []) {
      const name = reference.name?.trim() ?? "";
      const description = reference.description?.trim() ?? "";
      if (!description) {
        continue;
      }
      const key = `${name}\u0000${description}`;
      if (seen.has(key)) {
        continue;
      }
      seen.add(key);
      references.push({ description, name });
    }
  }
  return references;
}

function collectVisibleToolUseIds(projection: {
  directOrderedProjection: { content: readonly ContentBlock[] };
  finalAssistantContent: string | readonly ContentBlock[] | null;
  processProjection: { content: readonly ContentBlock[] };
}): Set<string> {
  const ids = new Set<string>();
  const collect = (content: readonly ContentBlock[]): void => {
    for (const block of content) {
      if (block.type === "tool_use") {
        ids.add(block.id);
      }
    }
  };
  collect(projection.directOrderedProjection.content);
  collect(projection.processProjection.content);
  if (
    projection.finalAssistantContent
    && typeof projection.finalAssistantContent !== "string"
  ) {
    collect(projection.finalAssistantContent);
  }
  return ids;
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

function shouldIncludeTransientSystemEvents(
  isLastRound: boolean | undefined,
  isLoading: boolean | undefined,
): boolean {
  return [isLastRound, isLoading].every(Boolean);
}

function projectAssistantIdentity(
  messages: AssistantMessage[],
  identityAssistant: AssistantMessage | undefined,
): AssistantIdentityProjection {
  if (!identityAssistant) {
    return {
      assistantAgentId: null,
      automationTaskName: null,
      echo: false,
      firstAssistantMessageId: null,
      model: undefined,
      stopReason: null,
      streamStatus: null,
    };
  }
  return {
    assistantAgentId: identityAssistant.agent_id ?? null,
    automationTaskName: resolveAutomationTaskName(identityAssistant),
    echo: identityAssistant.metadata?.source === "echo",
    firstAssistantMessageId: messages[0]?.message_id ?? null,
    model: resolveAssistantModel(messages, identityAssistant),
    stopReason: identityAssistant.stop_reason ?? null,
    streamStatus: identityAssistant.stream_status ?? null,
  };
}

function resolveAutomationTaskName(message: AssistantMessage): string | null {
  if (message.metadata?.source !== "automation_delivery") {
    return null;
  }
  const taskName = message.metadata.task_name;
  return typeof taskName === "string" && taskName.trim() ? taskName.trim() : "";
}

function selectAssistantIdentity(
  messages: AssistantMessage[],
): AssistantMessage | undefined {
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const message = messages[index];
    if (
      message.delivery_mode !== "ephemeral"
      && (message.result_summary || message.stop_reason || message.is_complete)
    ) {
      return message;
    }
  }
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    if (messages[index].delivery_mode !== "ephemeral") {
      return messages[index];
    }
  }
  return messages.at(-1);
}

export function resolveAssistantModel(
  messages: AssistantMessage[],
  identity: AssistantMessage,
): string | undefined {
  if (identity.metadata?.subtype === "conversation_welcome") {
    return undefined;
  }
  if (identity.model?.trim()) {
    return identity.model;
  }
  for (let index = messages.length - 1; index >= 0; index -= 1) {
    const model = messages[index].model?.trim();
    if (model) {
      return model;
    }
  }
  return undefined;
}

function resolveMessageTimestamp(
  identityAssistant: AssistantMessage | undefined,
  firstSystemEventTimestamp: number | undefined,
  resultSummary: ResultSummary | undefined,
): number | undefined {
  return resultSummary?.timestamp
    ?? identityAssistant?.timestamp
    ?? firstSystemEventTimestamp;
}
