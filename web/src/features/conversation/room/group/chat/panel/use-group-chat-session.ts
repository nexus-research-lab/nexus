import { useMemo, useRef } from "react";

import { useAgentConversation } from "@/hooks/agent";
import { useFollowScroll } from "@/hooks/conversation/use-follow-scroll";
import { useSessionLoader } from "@/hooks/conversation/use-session-loader";
import { useSessionRoundIndex } from "@/hooks/conversation/use-session-round-index";
import type {
  AgentConversationIdentity,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import type { ConversationRoundScrollHandle } from "@/features/conversation/shared/timeline/round-scroll";
import { buildConversationScrollContentKey } from "@/features/conversation/shared/timeline/scroll-content-key";
import { useConversationHistoryLoader } from "@/features/conversation/shared/timeline/use-history-loader";
import { useConversationTimeline } from "@/features/conversation/shared/timeline/use-conversation-timeline";
import { useVisibleRoundWindowLoader } from "@/features/conversation/shared/timeline/use-visible-window-loader";

interface UseGroupChatSessionOptions {
  identity: AgentConversationIdentity | null;
  onRoomEvent: (eventType: string, data: RoomEventPayload) => void;
  sessionKey: string | null;
}

export function useGroupChatSession({
  identity,
  onRoomEvent,
  sessionKey,
}: UseGroupChatSessionOptions) {
  const roundScrollRef = useRef<ConversationRoundScrollHandle | null>(null);
  const conversation = useAgentConversation({
    identity,
    on_error: (error) => {
      console.error("Room conversation error:", error);
    },
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
    debug_name: "GroupChatPanel",
    load_session: conversation.load_session,
    session_key: sessionKey,
  });

  const roundIndexItems = useSessionRoundIndex(sessionKey);
  const timeline = useConversationTimeline({
    chat_type: "group",
    live_round_ids: conversation.live_round_ids,
    messages: conversation.messages,
    pending_agent_slots: conversation.pending_agent_slots,
    pending_permissions: conversation.pending_permissions,
    round_index_items: roundIndexItems,
  });
  const useIndexedTimeline = roundIndexItems.length > 0;
  useVisibleRoundWindowLoader({
    enabled: useIndexedTimeline,
    loadRoundWindow: conversation.load_round_window,
    revision: buildVisibleRoundRevision(conversation, timeline.feed_round_ids),
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
    timeline,
  };
}

function buildVisibleRoundRevision(
  conversation: ReturnType<typeof useAgentConversation>,
  feedRoundIds: string[],
): string {
  return [
    feedRoundIds.length,
    conversation.messages.length,
    conversation.pending_agent_slots.length,
    conversation.pending_permissions.length,
    conversation.live_round_ids.length,
  ].join(":");
}
