/**
 * INPUT: 会话历史、初始内容锚点、slot/权限/execution 运行态与 round 索引。
 * OUTPUT: feed、navigator、可顶部起始的滚动、历史窗口加载反馈与当前轮次共享的 session 视图状态。
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
import { useConversationTimeline } from "../timeline/use-conversation-timeline";
import { useContinuousWindowLoader } from "../timeline/window-loader/use-continuous-window-loader";

interface UseConversationSessionOptions {
  chatType: AgentConversationChatType;
  debugName: string;
  identity: AgentConversationIdentity | null;
  initialScrollAnchor?: "bottom" | "top";
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
  visibleAfterUnixMilli?: number;
}

export function useConversationSession({
  chatType,
  debugName,
  identity,
  initialScrollAnchor,
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
    () => buildConversationAtomicLayoutKey(
      sessionKey,
      visibleMessages,
      conversation.pending_permissions,
    ),
    [
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
    initialScrollAnchor,
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

  const roundIndexResource = useSessionRoundIndex(sessionKey);
  const timeline = useConversationTimeline({
    chat_type: chatType,
    live_round_ids: conversation.live_round_ids,
    messages: visibleMessages,
    pending_agent_slots: conversation.pending_agent_slots,
    pending_permissions: conversation.pending_permissions,
    room_agent_execution_states: conversation.room_agent_execution_states,
    resolved_history_round_ids: conversation.resolved_history_round_ids,
    round_index_items: roundIndexResource.items,
  });
  const roundIndexItems = timeline.round_index_items;
  const feedRoundIds = timeline.feed_round_ids;
  const windowLoader = useContinuousWindowLoader({
    enabled: Boolean(sessionKey),
    feedRef: scroll.feedRef,
    gapAfterRoundId: conversation.history_gap_after_round_id,
    hasOlderHistory: conversation.has_more_history,
    isBottomScrollActive: scroll.isBottomScrollActive,
    isFollowingLatest: scroll.isFollowingLatest,
    isSessionLoading: conversation.is_session_loading,
    loadNewerMessages: conversation.load_newer_messages,
    loadOlderMessages: conversation.load_older_messages,
    returnToLatestMessages: conversation.return_to_latest_messages,
    revision: `${feedRoundIds.length}:${feedRoundIds[0] ?? ""}:${feedRoundIds.at(-1) ?? ""}`,
    scopeKey: sessionKey,
    scrollRef: scroll.scrollRef,
    setEvictionEdge: conversation.set_history_eviction_edge,
    setWindowChangePending: scroll.setWindowChangePending,
    windowRoundIds: feedRoundIds,
  });
  const followScrollToBottom = scroll.scrollToBottom;
  const hasHistoryGap = conversation.history_gap_after_round_id !== null;
  const returnToLatestMessages = conversation.return_to_latest_messages;
  const scrollToBottom = useCallback((behavior?: ScrollBehavior) => {
    if (hasHistoryGap) {
      // 驻留窗口已脱离最新：先把最新一页接回来，FOLLOW 随提交贴到真实底部。
      void returnToLatestMessages();
    }
    followScrollToBottom(behavior);
  }, [followScrollToBottom, hasHistoryGap, returnToLatestMessages]);
  const visibleConversation = useMemo(
    () => ({
      ...conversation,
      is_history_loading:
        conversation.is_history_loading || windowLoader.isLoading,
      messages: visibleMessages,
    }),
    [conversation, visibleMessages, windowLoader.isLoading],
  );

  return {
    conversation: visibleConversation,
    roundIndexResource,
    roundIndexItems,
    roundScrollRef,
    scroll: { ...scroll, scrollToBottom },
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
