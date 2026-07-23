import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
  type WheelEvent as ReactWheelEvent,
} from "react";

import { CONVERSATION_TABS_VIEWPORT_INSET } from "./conversation-tabs-model";

const SCROLL_EDGE_TOLERANCE = 2;
const DRAG_START_THRESHOLD = 4;
const TAB_WIDTH_TRANSITION_SETTLE_MS = 170;

export interface ConversationTabsScrollMetrics {
  clientWidth: number;
  maxScrollLeft: number;
  scrollLeft: number;
  scrollWidth: number;
}

const EMPTY_SCROLL_METRICS: ConversationTabsScrollMetrics = {
  clientWidth: 0,
  maxScrollLeft: 0,
  scrollLeft: 0,
  scrollWidth: 0,
};

interface ConversationTabsDragState {
  pointerId: number;
  scrollLeft: number;
  startX: number;
}

export function useConversationTabsScroll({
  activeConversationId,
  contentKey,
}: {
  activeConversationId: string | null;
  contentKey: string;
}) {
  const viewportRef = useRef<HTMLDivElement | null>(null);
  const dragStateRef = useRef<ConversationTabsDragState | null>(null);
  const suppressClickRef = useRef(false);
  const [isDragging, setIsDragging] = useState(false);
  const [metrics, setMetrics] = useState(EMPTY_SCROLL_METRICS);

  const updateMetrics = useCallback(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return;
    }
    const maxScrollLeft = Math.max(0, viewport.scrollWidth - viewport.clientWidth);
    const nextMetrics = {
      clientWidth: viewport.clientWidth,
      maxScrollLeft,
      scrollLeft: Math.min(Math.max(viewport.scrollLeft, 0), maxScrollLeft),
      scrollWidth: viewport.scrollWidth,
    };
    setMetrics((current) => (
      current.clientWidth === nextMetrics.clientWidth
      && current.maxScrollLeft === nextMetrics.maxScrollLeft
      && current.scrollLeft === nextMetrics.scrollLeft
      && current.scrollWidth === nextMetrics.scrollWidth
        ? current
        : nextMetrics
    ));
  }, []);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport) {
      return undefined;
    }

    const frame = window.requestAnimationFrame(updateMetrics);
    viewport.addEventListener("scroll", updateMetrics, { passive: true });
    const resizeObserver = new ResizeObserver(updateMetrics);
    resizeObserver.observe(viewport);
    const content = viewport.firstElementChild;
    if (content) {
      resizeObserver.observe(content);
    }

    return () => {
      window.cancelAnimationFrame(frame);
      viewport.removeEventListener("scroll", updateMetrics);
      resizeObserver.disconnect();
    };
  }, [contentKey, updateMetrics]);

  useLayoutEffect(() => {
    const viewport = viewportRef.current;
    if (!viewport || !activeConversationId) {
      return undefined;
    }
    const preferredAlignment = getConversationTabAlignment(
      viewport,
      activeConversationId,
    );
    const alignActiveTab = (behavior: ScrollBehavior) => {
      scrollConversationTabIntoView(
        viewport,
        activeConversationId,
        behavior,
        preferredAlignment,
      );
    };
    const frame = window.requestAnimationFrame(() => alignActiveTab("smooth"));
    // 中文注释：标签宽度会平滑交换，动画结束后按最终尺寸再校正一次边界。
    const settleTimer = window.setTimeout(
      () => alignActiveTab("auto"),
      TAB_WIDTH_TRANSITION_SETTLE_MS,
    );
    return () => {
      window.cancelAnimationFrame(frame);
      window.clearTimeout(settleTimer);
    };
  }, [activeConversationId, contentKey]);

  const setScrollLeft = useCallback((scrollLeft: number) => {
    viewportRef.current?.scrollTo({ left: scrollLeft });
  }, []);

  const handlePointerDown = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || event.pointerType !== "mouse") {
      return;
    }
    dragStateRef.current = {
      pointerId: event.pointerId,
      scrollLeft: event.currentTarget.scrollLeft,
      startX: event.clientX,
    };
    suppressClickRef.current = false;
  }, []);

  const finishDragging = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }
    if (event.currentTarget.hasPointerCapture(event.pointerId)) {
      event.currentTarget.releasePointerCapture(event.pointerId);
    }
    dragStateRef.current = null;
    setIsDragging(false);
    window.requestAnimationFrame(() => {
      suppressClickRef.current = false;
    });
  }, []);

  const handlePointerMove = useCallback((event: ReactPointerEvent<HTMLDivElement>) => {
    const dragState = dragStateRef.current;
    if (!dragState || dragState.pointerId !== event.pointerId) {
      return;
    }
    const deltaX = event.clientX - dragState.startX;
    if (!suppressClickRef.current && Math.abs(deltaX) < DRAG_START_THRESHOLD) {
      return;
    }
    if (!suppressClickRef.current) {
      suppressClickRef.current = true;
      event.currentTarget.setPointerCapture(event.pointerId);
      setIsDragging(true);
    }
    event.preventDefault();
    event.currentTarget.scrollLeft = dragState.scrollLeft - deltaX;
  }, []);

  const handleClickCapture = useCallback((event: ReactMouseEvent<HTMLDivElement>) => {
    if (!suppressClickRef.current) {
      return;
    }
    event.preventDefault();
    event.stopPropagation();
  }, []);

  const handleWheel = useCallback((event: ReactWheelEvent<HTMLDivElement>) => {
    const viewport = viewportRef.current;
    if (
      !viewport
      || viewport.scrollWidth <= viewport.clientWidth + SCROLL_EDGE_TOLERANCE
      || Math.abs(event.deltaX) >= Math.abs(event.deltaY)
    ) {
      return;
    }
    event.preventDefault();
    viewport.scrollLeft += event.deltaY;
  }, []);

  return {
    handleClickCapture,
    handlePointerCancel: finishDragging,
    handlePointerDown,
    handlePointerMove,
    handlePointerUp: finishDragging,
    handleWheel,
    isDragging,
    metrics,
    setScrollLeft,
    viewportRef: viewportRef as RefObject<HTMLDivElement | null>,
  };
}

function scrollConversationTabIntoView(
  viewport: HTMLDivElement,
  activeConversationId: string,
  behavior: ScrollBehavior,
  preferredAlignment: ConversationTabAlignment,
): void {
  const activeTab = findConversationTab(viewport, activeConversationId);
  if (!activeTab) {
    return;
  }

  const viewportLeft = viewport.scrollLeft + CONVERSATION_TABS_VIEWPORT_INSET;
  const viewportRight = viewport.scrollLeft
    + viewport.clientWidth
    - CONVERSATION_TABS_VIEWPORT_INSET;
  const tabLeft = activeTab.offsetLeft;
  const tabRight = tabLeft + activeTab.offsetWidth;
  if (preferredAlignment === "start" || tabLeft < viewportLeft) {
    viewport.scrollTo({
      behavior,
      left: tabLeft - CONVERSATION_TABS_VIEWPORT_INSET,
    });
  } else if (preferredAlignment === "end" || tabRight > viewportRight) {
    viewport.scrollTo({
      behavior,
      left: tabRight
        - viewport.clientWidth
        + CONVERSATION_TABS_VIEWPORT_INSET,
    });
  }
}

type ConversationTabAlignment = "end" | "start" | null;

function getConversationTabAlignment(
  viewport: HTMLDivElement,
  activeConversationId: string,
): ConversationTabAlignment {
  const activeTab = findConversationTab(viewport, activeConversationId);
  if (!activeTab) {
    return null;
  }
  if (
    activeTab.offsetLeft
    < viewport.scrollLeft + CONVERSATION_TABS_VIEWPORT_INSET
  ) {
    return "start";
  }
  if (
    activeTab.offsetLeft + activeTab.offsetWidth
    > viewport.scrollLeft
      + viewport.clientWidth
      - CONVERSATION_TABS_VIEWPORT_INSET
  ) {
    return "end";
  }
  return null;
}

function findConversationTab(
  viewport: HTMLDivElement,
  conversationId: string,
): HTMLElement | undefined {
  return Array.from(
    viewport.querySelectorAll<HTMLElement>("[data-conversation-tab-id]"),
  ).find((tab) => tab.dataset.conversationTabId === conversationId);
}
