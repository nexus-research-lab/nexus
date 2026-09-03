/**
 * INPUT: 会话、时间线、历史、导航、滚动与 Composer 子模型。
 * OUTPUT: 主对话面板各视图区域消费的稳定 props 投影。
 * POS: 会话控制器与纯视图布局之间的共享模型装配层。
 */
import type {
  ComponentProps,
  PointerEvent,
  RefObject,
  TouchEvent,
  WheelEvent,
} from "react";

import type { UseAgentConversationReturn } from "@/types/agent/agent-conversation";
import type { SessionRoundIndexResource } from "@/hooks/conversation/use-session-round-index";

import type { ConversationSessionNavigator } from "./session-navigator/conversation-session-navigator";
import type {
  ConversationScrollToLatestModel,
  ConversationViewportModel,
} from "./conversation-panel-layout";
import type { ConversationTimeline } from "./timeline/timeline-model";
import type { ConversationRoundScrollHandle } from "./timeline/scroll/round-scroll";

interface ConversationPanelScrollSource {
  onPointerDown: (event: PointerEvent<HTMLDivElement>) => void;
  onTouchEnd: () => void;
  onTouchMove: (event: TouchEvent<HTMLDivElement>) => void;
  onTouchStart: (event: TouchEvent<HTMLDivElement>) => void;
  onWheel: (event: WheelEvent<HTMLDivElement>) => void;
  pauseFollowLatest: () => void;
  scrollRef: RefObject<HTMLDivElement | null>;
  scrollToBottom: (behavior?: ScrollBehavior) => void;
  showScrollToBottom: boolean;
}

interface ConversationNavigatorSessionSource {
  conversation: Pick<
    UseAgentConversationReturn,
    "load_round_window" | "load_session"
  >;
  roundScrollRef: RefObject<ConversationRoundScrollHandle | null>;
  scroll: Pick<
    ConversationPanelScrollSource,
    "pauseFollowLatest" | "scrollRef"
  >;
  sessionKey: string | null;
  timeline: ConversationTimeline;
}

interface ConversationScrollToLatestSessionSource {
  scroll: Pick<
    ConversationPanelScrollSource,
    "scrollToBottom" | "showScrollToBottom"
  >;
}

interface ConversationRuntimeSessionSource {
  conversation: Pick<
    UseAgentConversationReturn,
    "is_loading" | "is_session_loading"
  >;
}

interface ConversationRoundIndexSessionSource {
  roundIndexResource: Pick<
    SessionRoundIndexResource,
    "access" | "error" | "isLoading" | "isStale" | "retry"
  >;
}

interface ConversationViewportSessionSource {
  conversation: Pick<
    UseAgentConversationReturn,
    "is_history_loading" | "reliability"
  >;
  history: {
    handleScroll: () => void;
  };
  scroll: Pick<
    ConversationPanelScrollSource,
    | "onPointerDown"
    | "onTouchEnd"
    | "onTouchMove"
    | "onTouchStart"
    | "onWheel"
    | "scrollRef"
  >;
}

export type ConversationPanelSessionSource =
  & ConversationNavigatorSessionSource
  & ConversationRoundIndexSessionSource
  & ConversationRuntimeSessionSource
  & ConversationScrollToLatestSessionSource
  & ConversationViewportSessionSource;

type ConversationNavigatorModel = Omit<
  ComponentProps<typeof ConversationSessionNavigator>,
  "className"
>;

export interface ConversationPanelEnvironment {
  isMobileLayout: boolean;
  providerWarningVisible: boolean;
}

export interface ConversationPanelFrameModel {
  isMobileLayout: boolean;
  isSessionLoading: boolean;
  navigator: ConversationNavigatorModel;
  providerWarningVisible: boolean;
  reconcileConversation: () => void;
  reliability: UseAgentConversationReturn["reliability"];
  roundIndexResource: ConversationRoundIndexSessionSource["roundIndexResource"];
  scrollToLatest: ConversationScrollToLatestModel;
  sessionKey: string | null;
  viewport: ConversationViewportModel;
}

export function buildConversationPanelFrameModel(
  session: ConversationPanelSessionSource,
  environment: ConversationPanelEnvironment,
): ConversationPanelFrameModel {
  return {
    isMobileLayout: environment.isMobileLayout,
    isSessionLoading: session.conversation.is_session_loading,
    navigator: buildConversationNavigatorModel(session),
    providerWarningVisible: environment.providerWarningVisible,
    reconcileConversation: () => {
      if (session.sessionKey) {
        void session.conversation.load_session(session.sessionKey);
      }
    },
    reliability: session.conversation.reliability,
    roundIndexResource: session.roundIndexResource,
    scrollToLatest: buildConversationScrollToLatestModel(session),
    sessionKey: session.sessionKey,
    viewport: buildConversationViewportModel(session),
  };
}

function buildConversationNavigatorModel(
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

function buildConversationScrollToLatestModel(
  session: ConversationScrollToLatestSessionSource & ConversationRuntimeSessionSource,
): ConversationScrollToLatestModel {
  return {
    isGenerating: session.conversation.is_loading,
    onClick: () => session.scroll.scrollToBottom("smooth"),
    visible: session.conversation.is_loading || session.scroll.showScrollToBottom,
  };
}

function buildConversationViewportModel(
  session: ConversationViewportSessionSource,
): ConversationViewportModel {
  const { conversation, history, scroll } = session;
  return {
    isHistoryLoading: conversation.is_history_loading,
    onPointerDown: scroll.onPointerDown,
    onScroll: history.handleScroll,
    onTouchEnd: scroll.onTouchEnd,
    onTouchMove: scroll.onTouchMove,
    onTouchStart: scroll.onTouchStart,
    onWheel: scroll.onWheel,
    scrollRef: scroll.scrollRef,
  };
}
