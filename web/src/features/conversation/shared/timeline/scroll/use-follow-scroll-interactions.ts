import {
  useCallback,
  useRef,
  type MutableRefObject,
  type RefObject,
  type TouchEvent,
  type WheelEvent,
} from "react";

interface UseFollowScrollInteractionsOptions {
  cancelAnimation: () => void;
  lastScrollTopRef: MutableRefObject<number>;
  scrollRef: RefObject<HTMLDivElement | null>;
  updateFollowState: () => void;
}

export function useFollowScrollInteractions({
  cancelAnimation,
  lastScrollTopRef,
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
    if (currentScrollTop < lastScrollTopRef.current) {
      cancelAnimation();
    }
    lastScrollTopRef.current = currentScrollTop;
    updateFollowState();
  }, [cancelAnimation, lastScrollTopRef, scrollRef, updateFollowState]);

  const onWheel = useCallback(
    (event: WheelEvent<HTMLDivElement>) => {
      if (event.deltaY < 0) {
        cancelAnimation();
      }
    },
    [cancelAnimation],
  );

  const onTouchStart = useCallback((event: TouchEvent<HTMLDivElement>) => {
    touchStartYRef.current = event.touches[0]?.clientY ?? null;
  }, []);

  const onTouchMove = useCallback(
    (event: TouchEvent<HTMLDivElement>) => {
      const currentY = event.touches[0]?.clientY;
      if (currentY !== undefined && touchStartYRef.current !== null) {
        if (currentY > touchStartYRef.current) {
          cancelAnimation();
        }
      }
    },
    [cancelAnimation],
  );

  const onTouchEnd = useCallback(() => {
    touchStartYRef.current = null;
  }, []);

  return { onScroll, onTouchEnd, onTouchMove, onTouchStart, onWheel };
}
