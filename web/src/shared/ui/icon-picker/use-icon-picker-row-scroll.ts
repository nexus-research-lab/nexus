import { useCallback, useEffect, useRef, useState } from "react";

interface IconPickerRowScrollMetrics {
  canScroll: boolean;
  canScrollBackward: boolean;
  canScrollForward: boolean;
  maxScrollLeft: number;
  scrollLeft: number;
}

const EMPTY_SCROLL_METRICS: IconPickerRowScrollMetrics = {
  canScroll: false,
  canScrollBackward: false,
  canScrollForward: false,
  maxScrollLeft: 0,
  scrollLeft: 0,
};

const SCROLL_EDGE_TOLERANCE = 1;
const PAGE_SCROLL_RATIO = 0.72;

function readScrollMetrics(element: HTMLDivElement): IconPickerRowScrollMetrics {
  const maxScrollLeft = Math.max(element.scrollWidth - element.clientWidth, 0);
  const scrollLeft = Math.min(Math.max(element.scrollLeft, 0), maxScrollLeft);
  return {
    canScroll: maxScrollLeft > SCROLL_EDGE_TOLERANCE,
    canScrollBackward: scrollLeft > SCROLL_EDGE_TOLERANCE,
    canScrollForward: scrollLeft < maxScrollLeft - SCROLL_EDGE_TOLERANCE,
    maxScrollLeft,
    scrollLeft,
  };
}

/** 横排图标的可见滑轨与原生滚动位置共用这一处测量和命令入口。 */
export function useIconPickerRowScroll({
  enabled,
  itemCount,
}: {
  enabled: boolean;
  itemCount: number;
}) {
  const collectionRef = useRef<HTMLDivElement | null>(null);
  const [metrics, setMetrics] = useState<IconPickerRowScrollMetrics>(
    EMPTY_SCROLL_METRICS,
  );

  const syncMetrics = useCallback(() => {
    const element = collectionRef.current;
    setMetrics(element && enabled
      ? readScrollMetrics(element)
      : EMPTY_SCROLL_METRICS);
  }, [enabled]);

  useEffect(() => {
    const element = collectionRef.current;
    if (!element || !enabled) {
      setMetrics(EMPTY_SCROLL_METRICS);
      return;
    }

    syncMetrics();
    element.addEventListener("scroll", syncMetrics, { passive: true });
    const resizeObserver = new ResizeObserver(syncMetrics);
    resizeObserver.observe(element);

    return () => {
      element.removeEventListener("scroll", syncMetrics);
      resizeObserver.disconnect();
    };
  }, [enabled, itemCount, syncMetrics]);

  const scrollByPage = useCallback((direction: -1 | 1) => {
    const element = collectionRef.current;
    if (!element) return;
    element.scrollBy({
      behavior: "smooth",
      left: direction * element.clientWidth * PAGE_SCROLL_RATIO,
    });
  }, []);

  const setScrollLeft = useCallback((scrollLeft: number) => {
    collectionRef.current?.scrollTo({ left: scrollLeft });
  }, []);

  return {
    collectionRef,
    metrics,
    scrollBackward: () => scrollByPage(-1),
    scrollForward: () => scrollByPage(1),
    setScrollLeft,
  };
}
