import type { ComponentProps } from "react";

import type { ConversationSessionNavigator } from "./session-navigator/conversation-session-navigator";
import type { useConversationSession } from "./session/use-conversation-session";
import type {
  ConversationScrollToLatestModel,
  ConversationViewportModel,
} from "./conversation-panel-layout";

type ConversationSession = ReturnType<typeof useConversationSession>;

export type ConversationNavigatorModel = Omit<
  ComponentProps<typeof ConversationSessionNavigator>,
  "className"
>;

export function buildConversationNavigatorModel(
  session: ConversationSession,
): ConversationNavigatorModel {
  const { conversation, roundScrollRef, scroll, sessionKey, timeline } = session;
  return {
    onLoadRoundWindow: conversation.load_round_window,
    onNavigateStart: scroll.pauseFollowLatest,
    roundScrollRef,
    scopeKey: sessionKey,
    scrollRef: scroll.scrollRef,
    timeline,
  };
}

export function buildConversationScrollToLatestModel(
  session: ConversationSession,
): ConversationScrollToLatestModel {
  return {
    isLoading: session.conversation.is_loading,
    onClick: () => session.scroll.scrollToBottom("smooth"),
    visible: session.scroll.showScrollToBottom,
  };
}

export function buildConversationViewportModel(
  session: ConversationSession,
): ConversationViewportModel {
  const { conversation, history, scroll } = session;
  return {
    error: conversation.error,
    isHistoryLoading: conversation.is_history_loading,
    onScroll: history.handleScroll,
    onTouchEnd: scroll.onTouchEnd,
    onTouchMove: scroll.onTouchMove,
    onTouchStart: scroll.onTouchStart,
    onWheel: scroll.onWheel,
    scrollRef: scroll.scrollRef,
  };
}
