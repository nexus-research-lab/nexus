/**
 * INPUT: 面板状态、内容节点、滚动 refs、会话导航、Goal、可靠性快照、底部活动入口与统一输入事件。
 * OUTPUT: 可聚焦的主对话滚动布局、只约束在 viewport 内的导航，以及承载可靠性状态和活动组件的 Composer 底部工作栈。
 * POS: DM 与 Room 主对话面板的共享纯视图骨架。
 */
import type { ComponentProps, ReactNode, RefObject } from "react";

import type { SessionRoundIndexResource } from "@/hooks/conversation/use-session-round-index";
import { useI18n } from "@/shared/i18n/i18n-context";

import { ConversationReliabilityNotice } from "./conversation-reliability-notice";
import {
  CONVERSATION_COMPOSER_LANE_CLASS_NAME,
  CONVERSATION_CONTENT_LANE_CLASS_NAME,
} from "./conversation-panel-styles";
import { ProviderUnavailableBanner } from "./provider-unavailable-banner";
import { ReadResourceReliabilityNotice } from "./read-resource-reliability-notice";
import { ScrollToLatestButton } from "./scroll-to-latest-button";

type ScrollViewportEvents = Pick<
  ComponentProps<"div">,
  | "onPointerDown"
  | "onScroll"
  | "onTouchEnd"
  | "onTouchMove"
  | "onTouchStart"
  | "onWheel"
>;

export type ConversationViewportModel = ScrollViewportEvents & {
  isHistoryLoading: boolean;
  scrollRef: RefObject<HTMLDivElement | null>;
};

export interface ConversationScrollToLatestModel {
  onClick: () => void;
  visible: boolean;
}

export function ConversationPanelLayout({ children }: { children: ReactNode }) {
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {children}
    </div>
  );
}

export function ConversationPanelViewportArea({
  children,
  navigator,
}: {
  children: ReactNode;
  navigator?: ReactNode;
}) {
  return (
    <div className="relative flex min-h-0 min-w-0 flex-1">
      {children}
      {navigator}
    </div>
  );
}

export function ConversationPanelViewport({
  children,
  floatingDockOccupied,
  isMobileLayout,
  tourAnchor,
  viewport,
}: {
  children: ReactNode;
  floatingDockOccupied: boolean;
  isMobileLayout: boolean;
  tourAnchor?: string;
  viewport: ConversationViewportModel;
}) {
  const { t } = useI18n();
  return (
    <div
      data-tour-anchor={tourAnchor}
      ref={viewport.scrollRef}
      className={
        isMobileLayout
          ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2 outline-none"
          : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-3 py-4 outline-none sm:px-5 sm:py-5 xl:px-7 xl:py-5"
      }
      style={{ overflowAnchor: "none", scrollbarGutter: "stable" }}
      tabIndex={-1}
      onPointerDown={viewport.onPointerDown}
      onScroll={viewport.onScroll}
      onTouchEnd={viewport.onTouchEnd}
      onTouchMove={viewport.onTouchMove}
      onTouchStart={viewport.onTouchStart}
      onWheel={viewport.onWheel}
    >
      {viewport.isHistoryLoading ? (
        <div
          className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} pointer-events-none sticky top-2 z-20 flex h-0 justify-center`}
          data-conversation-history-loading-overlay
        >
          <span className="inline-flex h-6 items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) px-2.5 text-xs text-muted-foreground shadow-(--surface-control-shadow)">
            {t("room.loading_earlier_messages")}
          </span>
        </div>
      ) : null}
      {children}
      {floatingDockOccupied ? (
        <div
          aria-hidden="true"
          className="h-14"
          data-conversation-dock-clearance
        />
      ) : null}
    </div>
  );
}

export function ConversationPanelFloatingControls({
  activity,
  isMobileLayout,
  scrollToLatest,
}: {
  activity?: ReactNode;
  isMobileLayout: boolean;
  scrollToLatest: ConversationScrollToLatestModel;
}) {
  return (
    <div
      className={
        isMobileLayout
          ? "pointer-events-none absolute inset-x-0 top-0 z-30 mx-auto flex min-h-11 w-full max-w-[720px] -translate-y-[calc(100%+0.5rem)] items-center justify-center gap-1 px-4"
          : "pointer-events-none absolute inset-x-0 top-0 z-30 mx-auto flex min-h-11 w-full max-w-[880px] -translate-y-[calc(100%+0.5rem)] items-center justify-center gap-1 px-3 sm:px-5 xl:px-6"
      }
      data-conversation-activity-dock
    >
      {activity ? (
        <div
          className="pointer-events-none flex min-w-0 flex-1 justify-center"
          data-conversation-dock-activity
        >
          {activity}
        </div>
      ) : null}
      <div
        className="pointer-events-none shrink-0"
        data-conversation-dock-scroll
      >
        <ScrollToLatestButton
          onClick={scrollToLatest.onClick}
          visible={scrollToLatest.visible}
        />
      </div>
    </div>
  );
}

export function ConversationPanelBottomArea({
  activity,
  children,
  goal,
  isMobileLayout,
  providerWarningVisible,
  reliability,
  roundIndexResource,
  scrollToLatest,
}: {
  activity?: ReactNode;
  children: ReactNode;
  goal?: ReactNode;
  isMobileLayout: boolean;
  providerWarningVisible: boolean;
  reliability: ComponentProps<typeof ConversationReliabilityNotice>["reliability"];
  roundIndexResource?: Pick<
    SessionRoundIndexResource,
    "access" | "error" | "isLoading" | "isStale" | "retry"
  >;
  scrollToLatest: ConversationScrollToLatestModel;
}) {
  const { t } = useI18n();
  return (
    <div
      className="relative z-10 shrink-0"
      data-conversation-bottom-area
    >
      <div
        className="relative"
        data-conversation-bottom-stack
      >
        <ConversationPanelFloatingControls
          activity={activity}
          isMobileLayout={isMobileLayout}
          scrollToLatest={scrollToLatest}
        />
        <div data-conversation-status-stack>
          {roundIndexResource?.error ? (
            <div
              className={isMobileLayout
                ? "px-4 pt-1"
                : `${CONVERSATION_COMPOSER_LANE_CLASS_NAME} px-3 pt-1 sm:px-5 xl:px-6`}
            >
              <ReadResourceReliabilityNotice
                className="rounded-[10px] border"
                impact={t(roundIndexResource.access
                  ? "conversation.round_index_access_impact"
                  : roundIndexResource.isStale
                  ? "conversation.round_index_stale_impact"
                  : "conversation.round_index_unavailable_impact")}
                isRefreshing={roundIndexResource.isLoading}
                nextStep={t(roundIndexResource.access
                  ? "conversation.round_index_access_next_step"
                  : "conversation.round_index_next_step")}
                onRefresh={roundIndexResource.retry}
                problem={t("conversation.round_index_refresh_failed")}
                resource="session-round-index"
                stale={roundIndexResource.isStale}
              />
            </div>
          ) : null}
          <ConversationReliabilityNotice
            compact={isMobileLayout}
            reliability={reliability}
          />
          {providerWarningVisible ? (
            <ProviderUnavailableBanner compact={isMobileLayout} />
          ) : null}
          {goal}
        </div>
        <div data-conversation-composer-anchor>
          {children}
        </div>
      </div>
    </div>
  );
}
