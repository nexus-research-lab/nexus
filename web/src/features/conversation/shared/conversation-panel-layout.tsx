/**
 * INPUT: 面板状态、内容节点、滚动 refs 与统一输入事件。
 * OUTPUT: 可聚焦且关闭浏览器锚点争抢的主对话滚动布局。
 * POS: DM 与 Room 主对话面板的共享纯视图骨架。
 */
import type { ComponentProps, ReactNode, RefObject } from "react";

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
  isLoading: boolean;
  onClick: () => void;
  visible: boolean;
}

export function ConversationPanelLayout({
  children,
  navigator,
}: {
  children: ReactNode;
  navigator?: ReactNode;
}) {
  return (
    <div className="relative flex h-full min-w-0 flex-1 flex-col overflow-hidden bg-transparent">
      {navigator}
      {children}
    </div>
  );
}

export function ConversationPanelViewport({
  children,
  isMobileLayout,
  tourAnchor,
  viewport,
}: {
  children: ReactNode;
  isMobileLayout: boolean;
  tourAnchor?: string;
  viewport: ConversationViewportModel;
}) {
  return (
    <div
      data-tour-anchor={tourAnchor}
      ref={viewport.scrollRef}
      className={
        isMobileLayout
          ? "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-1 py-2"
          : "soft-scrollbar relative z-0 min-w-0 flex-1 overflow-x-hidden overflow-y-auto px-3 py-4 sm:px-5 sm:py-5 xl:px-7 xl:py-5"
      }
      style={{ overflowAnchor: "none" }}
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
          正在加载更早消息...
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
    </div>
  );
}

export function ConversationPanelFloatingControls({
  isMobileLayout,
  providerWarningVisible,
  scrollToLatest,
}: {
  isMobileLayout: boolean;
  providerWarningVisible: boolean;
  scrollToLatest: ConversationScrollToLatestModel;
}) {
  return (
    <>
      <ScrollToLatestButton
        isLoading={scrollToLatest.isLoading}
        onClick={scrollToLatest.onClick}
        visible={scrollToLatest.visible}
      />
      {providerWarningVisible ? (
        <ProviderUnavailableBanner compact={isMobileLayout} />
      ) : null}
    </>
  );
}
