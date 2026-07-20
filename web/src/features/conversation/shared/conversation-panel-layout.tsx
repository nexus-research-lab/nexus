import type { ComponentProps, ReactNode, RefObject } from "react";

import { ConversationErrorBubble } from "./conversation-error-bubble";
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
      onPointerDown={viewport.onPointerDown}
      onScroll={viewport.onScroll}
      onTouchEnd={viewport.onTouchEnd}
      onTouchMove={viewport.onTouchMove}
      onTouchStart={viewport.onTouchStart}
      onWheel={viewport.onWheel}
    >
      {viewport.isHistoryLoading ? (
        <div className="mx-auto mb-3 flex w-full max-w-[980px] items-center justify-center text-xs text-muted-foreground">
          正在加载更早消息...
        </div>
      ) : null}
      {children}
      {viewport.error ? (
        <div
          className={
            isMobileLayout ? "mt-4" : "mx-auto mt-2 w-full max-w-[980px]"
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
      {scrollToLatest.visible ? (
        <ScrollToLatestButton
          isLoading={scrollToLatest.isLoading}
          isMobileLayout={isMobileLayout}
          onClick={scrollToLatest.onClick}
        />
      ) : null}
      {providerWarningVisible ? (
        <ProviderUnavailableBanner compact={isMobileLayout} />
      ) : null}
    </>
  );
}
