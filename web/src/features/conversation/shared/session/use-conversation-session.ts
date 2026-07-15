/**
 * INPUT: 会话历史、运行态、权限与 round 索引。
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
import { buildConversationScrollContentKey } from "../timeline/scroll/follow-scroll-model";
import { useConversationHistoryLoader } from "../timeline/use-history-loader";
import { useConversationTimeline } from "../timeline/use-conversation-timeline";
import { useVisibleRoundWindowLoader } from "../timeline/window-loader/use-visible-window-loader";

interface UseConversationSessionOptions {
  chatType: AgentConversationChatType;
  debugName: string;
  identity: AgentConversationIdentity | null;
  onRoomEvent?: (eventType: string, data: RoomEventPayload) => void;
}

export function useConversationSession({
  chatType,
  debugName,
  identity,
  onRoomEvent,
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
  const scrollContentKey = useMemo(
    () => buildConversationScrollContentKey(sessionKey, conversation.messages),
    [conversation.messages, sessionKey],
  );
  const scroll = useFollowScroll({
    auxiliaryBlockCount:
      conversation.pending_agent_slots.length +
      conversation.pending_permissions.length,
    auxiliaryBlockKey: conversation.error,
    contentKey: scrollContentKey,
    historyPrependToken: conversation.history_prepend_token,
    isLoading: conversation.is_loading,
    messageCount: conversation.messages.length,
    sessionKey,
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
    messages: conversation.messages,
    pending_agent_slots: conversation.pending_agent_slots,
    pending_permissions: conversation.pending_permissions,
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
      messageCount: conversation.messages.length,
      pendingAgentSlotCount: conversation.pending_agent_slots.length,
      pendingPermissionCount: conversation.pending_permissions.length,
    }),
    scopeKey: sessionKey,
    scrollRef: scroll.scrollRef,
  });
  const history = useConversationHistoryLoader({
    autoFillViewport: !useIndexedTimeline,
    cancelHistoryPrependRestore: scroll.cancelHistoryPrependRestore,
    hasMoreHistory: conversation.has_more_history,
    isHistoryLoading: conversation.is_history_loading,
    isLoading: conversation.is_loading,
    loadOlderMessages: conversation.load_older_messages,
    messageCount: conversation.messages.length,
    onScroll: scroll.onScroll,
    prepareHistoryPrependRestore: scroll.prepareHistoryPrependRestore,
    scrollRef: scroll.scrollRef,
  });

  return {
    conversation,
    history,
    roundIndexItems,
    roundScrollRef,
    scroll,
    sessionKey,
    timeline,
  };
}

interface VisibleRoundRevisionInput {
  feedRoundCount: number;
  liveRoundCount: number;
  messageCount: number;
  pendingAgentSlotCount: number;
  pendingPermissionCount: number;
}

function buildVisibleRoundRevision({
  feedRoundCount,
  liveRoundCount,
  messageCount,
  pendingAgentSlotCount,
  pendingPermissionCount,
}: VisibleRoundRevisionInput): string {
  return [
    feedRoundCount,
    messageCount,
    pendingAgentSlotCount,
    pendingPermissionCount,
    liveRoundCount,
  ].join(":");
}
