/**
 * INPUT: 会话历史、slot/权限/execution 运行态与 round 索引。
 * OUTPUT: feed、navigator 与当前轮次共享的 session 视图状态。
 * POS: 会话页面消费统一时间线模型的 React 装配入口。
 */
import { useCallback, useMemo, useRef } from "react";

import { useAgentConversation } from "@/hooks/agent/use-agent-conversation";
import { useFollowScroll } from "@/features/conversation/shared/timeline/scroll/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useSessionRoundIndex } from "@/hooks/conversation/use-session-round-index";
import type {
  AgentConversationChatType,
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";

import type { ConversationRoundScrollHandle } from "../timeline/scroll/round-scroll";
import {
  buildConversationAtomicLayoutKey,
  buildConversationScrollContentKey,
  buildConversationScrollTopologyKey,
  isConversationLiveLayoutActive,
} from "../timeline/scroll/follow-scroll-model";
import { useConversationHistoryLoader } from "../timeline/use-history-loader";
import { useConversationTimeline } from "../timeline/use-conversation-timeline";
import { useVisibleRoundWindowLoader } from "../timeline/window-loader/use-visible-window-loader";

interface UseConversationSessionOptions {
  chatType: AgentConversationChatType;
  debugName: string;
  identity: AgentConversationIdentity | null;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  visibleAfterUnixMilli?: number;
}

export function useConversationSession({
  chatType,
  debugName,
  identity,
  onRoomEvent,
  visibleAfterUnixMilli,
}: UseConversationSessionOptions) {
  const sessionKey = identity?.session_key ?? null;
  const roundScrollRef = useRef<ConversationRoundScrollHandle | null>(null);
  const handleError = useCallback(
    (error: Error): void => {
      console.error(`${debugName} conversation error:`, error);
    },
    [debugName],
  );
  const conversation = useAgentConversation({
    identity,
    on_error: handleError,
    on_room_event: onRoomEvent,
  });
  const visibleMessages = useMemo(
    () => visibleAfterUnixMilli
      ? conversation.messages.filter((message) => messageTimestamp(message) >= visibleAfterUnixMilli)
      : conversation.messages,
    [conversation.messages, visibleAfterUnixMilli],
  );
  const visibleConversation = useMemo(
    () => ({ ...conversation, messages: visibleMessages }),
    [conversation, visibleMessages],
  );
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(sessionKey, visibleMessages),
    [sessionKey, visibleMessages],
  );
  const scrollTopologyKey = useMemo(
    () => buildConversationScrollTopologyKey(
      sessionKey,
      visibleMessages,
      conversation.pending_agent_slots,
      conversation.pending_permissions,
      conversation.room_agent_execution_states,
    ),
    [
      visibleMessages,
      conversation.pending_agent_slots,
      conversation.pending_permissions,
      conversation.room_agent_execution_states,
      sessionKey,
    ],
  );
  const atomicLayoutKey = useMemo(
    () => [
      buildConversationAtomicLayoutKey(
        sessionKey,
        visibleMessages,
        conversation.pending_permissions,
      ),
      conversation.error ?? "",
    ].join("\u001f"),
    [
      conversation.error,
      visibleMessages,
      conversation.pending_permissions,
      sessionKey,
    ],
  );
  const liveLayoutActive = useMemo(
    () => isConversationLiveLayoutActive({
      isLoading: conversation.is_loading,
      liveRoundIds: conversation.live_round_ids,
      messages: visibleMessages,
      pendingAgentSlots: conversation.pending_agent_slots,
      roomAgentExecutionStates: conversation.room_agent_execution_states,
      runtimePhase: conversation.runtime_phase,
    }),
    [
      conversation.is_loading,
      conversation.live_round_ids,
      visibleMessages,
      conversation.pending_agent_slots,
      conversation.room_agent_execution_states,
      conversation.runtime_phase,
    ],
  );
  const scroll = useFollowScroll({
    atomicLayoutKey,
    contentKey: scrollContentKey,
    historyPrependToken: conversation.history_prepend_token,
    liveLayoutActive,
    messageCount: visibleMessages.length,
    sessionKey,
    topologyKey: scrollTopologyKey,
  });

  useSessionLoader({
    debug_name: debugName,
    load_session: conversation.load_session,
    session_key: sessionKey,
  });

  const rawRoundIndexItems = useSessionRoundIndex(sessionKey);
  const timeline = useConversationTimeline({
    chat_type: chatType,
    live_round_ids: conversation.live_round_ids,
    messages: visibleMessages,
    pending_agent_slots: conversation.pending_agent_slots,
    pending_permissions: conversation.pending_permissions,
    room_agent_execution_states: conversation.room_agent_execution_states,
    resolved_history_round_ids: conversation.resolved_history_round_ids,
    round_index_items: rawRoundIndexItems,
  });
  const roundIndexItems = timeline.round_index_items;
  const useIndexedTimeline = roundIndexItems.length > 0;
  useVisibleRoundWindowLoader({
    enabled: useIndexedTimeline,
    loadRoundWindow: conversation.load_round_window,
    revision: buildVisibleRoundRevision({
      feedRoundCount: timeline.feed_round_ids.length,
      liveRoundCount: conversation.live_round_ids.length,
      messageCount: visibleMessages.length,
      pendingAgentSlotCount: conversation.pending_agent_slots.length,
      pendingPermissionCount: conversation.pending_permissions.length,
      roomAgentExecutionStateCount:
        conversation.room_agent_execution_states.length,
    }),
    scopeKey: sessionKey,
    scrollRef: scroll.scrollRef,
  });
  const history = useConversationHistoryLoader({
    cancelHistoryPrependRestore: scroll.cancelHistoryPrependRestore,
    enabled: !useIndexedTimeline,
    hasMoreHistory: conversation.has_more_history,
    isHistoryLoading: conversation.is_history_loading,
    isFollowingLatest: scroll.isFollowingLatest,
    isLoading: conversation.is_loading,
    loadOlderMessages: conversation.load_older_messages,
    messageCount: visibleMessages.length,
    onScroll: scroll.onScroll,
    prepareHistoryPrependRestore: scroll.prepareHistoryPrependRestore,
    scrollRef: scroll.scrollRef,
  });

  return {
    conversation: visibleConversation,
    history,
    roundIndexItems,
    roundScrollRef,
    scroll,
    sessionKey,
    timeline,
  };
}

function messageTimestamp(message: { timestamp?: unknown }): number {
  const value = typeof message.timestamp === "number"
    ? message.timestamp
    : Number(message.timestamp ?? 0);
  return Number.isFinite(value) ? value : 0;
}

interface VisibleRoundRevisionInput {
  feedRoundCount: number;
  liveRoundCount: number;
  messageCount: number;
  pendingAgentSlotCount: number;
  pendingPermissionCount: number;
  roomAgentExecutionStateCount: number;
}

function buildVisibleRoundRevision({
  feedRoundCount,
  liveRoundCount,
  messageCount,
  pendingAgentSlotCount,
  pendingPermissionCount,
  roomAgentExecutionStateCount,
}: VisibleRoundRevisionInput): string {
  return [
    feedRoundCount,
    messageCount,
    pendingAgentSlotCount,
    pendingPermissionCount,
    roomAgentExecutionStateCount,
    liveRoundCount,
  ].join(":");
}
