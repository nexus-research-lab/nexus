import type {
  ComponentProps,
  RefObject,
  TouchEvent,
  WheelEvent,
} from "react";

import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";

import type { ConversationSessionNavigator } from "./session-navigator/conversation-session-navigator";
import type {
  ConversationScrollToLatestModel,
  ConversationViewportModel,
} from "./conversation-panel-layout";
import type { ConversationTimeline } from "./timeline/timeline-model";
import type { ConversationRoundScrollHandle } from "./timeline/scroll/round-scroll";

interface ConversationPanelScrollSource {
  onTouchEnd: () => void;
  onTouchMove: (event: TouchEvent<HTMLDivElement>) => void;
  onTouchStart: (event: TouchEvent<HTMLDivElement>) => void;
  onWheel: (event: WheelEvent<HTMLDivElement>) => void;
  pauseFollowLatest: () => void;
  scrollRef: RefObject<HTMLDivElement | null>;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  showScrollToBottom: boolean;
}

export interface ConversationNavigatorSessionSource {
  conversation: Pick<
    UseAgentConversationReturn,
    "load_round_window"
  >;
  roundScrollRef: RefObject<ConversationRoundScrollHandle | null>;
  scroll: Pick<
    ConversationPanelScrollSource,
    "pauseFollowLatest" | "scrollRef"
  >;
  sessionKey: string | null;
  timeline: ConversationTimeline;
}

export interface ConversationScrollToLatestSessionSource {
  conversation: Pick<UseAgentConversationReturn, "is_loading">;
  scroll: Pick<
    ConversationPanelScrollSource,
    "scrollToBottom" | "showScrollToBottom"
  >;
}

export interface ConversationViewportSessionSource {
  conversation: Pick<
    UseAgentConversationReturn,
    "error" | "is_history_loading"
  >;
  history: {
    handleScroll: () => void;
  };
  scroll: Pick<
    ConversationPanelScrollSource,
    | "onTouchEnd"
    | "onTouchMove"
    | "onTouchStart"
    | "onWheel"
    | "scrollRef"
  >;
}

export type ConversationPanelSessionSource =
  & ConversationNavigatorSessionSource
  & ConversationScrollToLatestSessionSource
  & ConversationViewportSessionSource;

export type ConversationNavigatorModel = Omit<
  ComponentProps<typeof ConversationSessionNavigator>,
  "className"
>;

export function buildConversationNavigatorModel(
  session: ConversationNavigatorSessionSource,
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
  session: ConversationScrollToLatestSessionSource,
): ConversationScrollToLatestModel {
  return {
    isLoading: session.conversation.is_loading,
    onClick: () => session.scroll.scrollToBottom("smooth"),
    visible: session.scroll.showScrollToBottom,
  };
}

export function buildConversationViewportModel(
  session: ConversationViewportSessionSource,
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
