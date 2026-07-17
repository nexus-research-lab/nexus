import {
  useCallback,
  useRef,
  type MutableRefObject,
  type PointerEvent,
  type RefObject,
  type TouchEvent,
  type WheelEvent,
} from "react";

import { isNearScrollBottom } from "./follow-scroll-model";

interface UseFollowScrollInteractionsOptions {
  lastScrollTopRef: MutableRefObject<number>;
  pauseFollowLatest: () => void;
  scrollRef: RefObject<HTMLDivElement | null>;
  updateFollowState: () => void;
}

export function useFollowScrollInteractions({
  lastScrollTopRef,
  pauseFollowLatest,
  scrollRef,
  updateFollowState,
}: UseFollowScrollInteractionsOptions) {
  const touchStartYRef = useRef<number | null>(null);

  const onScroll = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }

    const currentScrollTop = container.scrollTop;
    const shouldResume = isNearScrollBottom(container);
    lastScrollTopRef.current = currentScrollTop;
    if (shouldResume) {
      updateFollowState();
    }
  }, [lastScrollTopRef, scrollRef, updateFollowState]);

  const onWheel = useCallback(
    (event: WheelEvent<HTMLDivElement>) => {
      if (event.deltaY < 0) {
        pauseFollowLatest();
      }
    },
    [pauseFollowLatest],
  );

  const onPointerDown = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      if (event.target === event.currentTarget) {
        pauseFollowLatest();
      }
    },
    [pauseFollowLatest],
  );

  const onTouchStart = useCallback((event: TouchEvent<HTMLDivElement>) => {
    touchStartYRef.current = event.touches[0]?.clientY ?? null;
  }, []);

  const onTouchMove = useCallback(
    (event: TouchEvent<HTMLDivElement>) => {
      const currentY = event.touches[0]?.clientY;
      if (currentY !== undefined && touchStartYRef.current !== null) {
        if (currentY > touchStartYRef.current) {
          pauseFollowLatest();
        }
      }
    },
    [pauseFollowLatest],
  );

  const onTouchEnd = useCallback(() => {
    touchStartYRef.current = null;
  }, []);

  return {
    onPointerDown,
    onScroll,
    onTouchEnd,
    onTouchMove,
    onTouchStart,
    onWheel,
  };
}
