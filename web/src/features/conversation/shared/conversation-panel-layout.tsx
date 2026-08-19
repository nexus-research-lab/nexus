/**
 * INPUT: 面板状态、内容节点、滚动 refs、会话导航、Goal、底部活动入口与统一输入事件。
 * OUTPUT: 可聚焦的主对话滚动布局、只约束在 viewport 内的导航，以及向活动组件下发本地可用宽度的 Composer 底部工作栈。
 * POS: DM 与 Room 主对话面板的共享纯视图骨架。
 */
import type { ComponentProps, ReactNode, RefObject } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

import { ConversationErrorBubble } from "./conversation-error-bubble";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "./conversation-panel-styles";
import { ProviderUnavailableBanner } from "./provider-unavailable-banner";
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
  error: string | null;
  isHistoryLoading: boolean;
  scrollRef: RefObject<HTMLDivElement | null>;
};

export interface ConversationScrollToLatestModel {
  direction: "above" | "below" | null;
  onClick: () => void;
  unreadCount: number;
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
        <div className={`${CONVERSATION_CONTENT_LANE_CLASS_NAME} mb-3 flex items-center justify-center text-xs text-muted-foreground`}>
          {t("room.loading_earlier_messages")}
        </div>
      ) : null}
      {children}
      {viewport.error ? (
        <div
          className={
            isMobileLayout
              ? "mt-4"
              : `${CONVERSATION_CONTENT_LANE_CLASS_NAME} mt-2`
          }
        >
          <ConversationErrorBubble
            compact={isMobileLayout}
            error={viewport.error}
          />
        </div>
      ) : null}
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
          direction={scrollToLatest.direction}
          onClick={scrollToLatest.onClick}
          unreadCount={scrollToLatest.unreadCount}
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
  scrollToLatest,
}: {
  activity?: ReactNode;
  children: ReactNode;
  goal?: ReactNode;
  isMobileLayout: boolean;
  providerWarningVisible: boolean;
  scrollToLatest: ConversationScrollToLatestModel;
}) {
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
