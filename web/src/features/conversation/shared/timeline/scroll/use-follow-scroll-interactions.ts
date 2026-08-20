/**
 * INPUT: 原生 wheel、pointer、touch、scroll 事件与最近滚动位置。
 * OUTPUT: 带方向和时效的用户滚动意图，以及暂停/恢复跟随动作。
 * POS: 跟随滚动 Hook 的手势边界，不直接实现滚动动画。
 */
import {
  useCallback,
  useEffect,
  useRef,
  type MutableRefObject,
  type PointerEvent,
  type RefObject,
  type TouchEvent,
  type WheelEvent,
} from "react";

import {
  resolveKeyboardFollowScrollIntent,
  resolveTouchFollowScrollIntent,
  shouldPauseFollowOnScroll,
  shouldResumeFollowOnScroll,
  type FollowScrollIntent,
} from "./follow-scroll-model";

const USER_SCROLL_INTENT_TIMEOUT_MS = 180;

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
  const pointerGestureRef = useRef(false);
  const scrollIntentRef = useRef<FollowScrollIntent | null>(null);
  const scrollIntentTimerRef = useRef<number | null>(null);
  const lastTouchYRef = useRef<number | null>(null);

  const isUserScrollActive = useCallback(() => (
    pointerGestureRef.current
    || scrollIntentRef.current !== null
    || lastTouchYRef.current !== null
  ), []);

  const clearTransientScrollIntent = useCallback(() => {
    if (scrollIntentTimerRef.current !== null) {
      window.clearTimeout(scrollIntentTimerRef.current);
      scrollIntentTimerRef.current = null;
    }
    scrollIntentRef.current = null;
  }, []);

  const recordTransientScrollIntent = useCallback(
    (intent: FollowScrollIntent) => {
      clearTransientScrollIntent();
      scrollIntentRef.current = intent;
      scrollIntentTimerRef.current = window.setTimeout(() => {
        scrollIntentTimerRef.current = null;
        scrollIntentRef.current = null;
      }, USER_SCROLL_INTENT_TIMEOUT_MS);
    },
    [clearTransientScrollIntent],
  );

  const onScroll = useCallback(() => {
    const container = scrollRef.current;
    if (!container) {
      return;
    }

    const currentScrollTop = container.scrollTop;
    const shouldPause = shouldPauseFollowOnScroll(
      container,
      lastScrollTopRef.current,
      pointerGestureRef.current || scrollIntentRef.current === "up",
    );
    const shouldResume = shouldResumeFollowOnScroll(
      container,
      lastScrollTopRef.current,
      pointerGestureRef.current || scrollIntentRef.current === "down",
    );
    lastScrollTopRef.current = currentScrollTop;
    if (shouldPause) {
      pauseFollowLatest();
      return;
    }
    if (shouldResume) {
      updateFollowState();
    }
  }, [
    lastScrollTopRef,
    pauseFollowLatest,
    scrollRef,
    updateFollowState,
  ]);

  const onWheel = useCallback(
    (event: WheelEvent<HTMLDivElement>) => {
      recordTransientScrollIntent(event.deltaY < 0 ? "up" : "down");
      if (event.deltaY < 0) {
        pauseFollowLatest();
      }
    },
    [pauseFollowLatest, recordTransientScrollIntent],
  );

  const onPointerDown = useCallback(
    (event: PointerEvent<HTMLDivElement>) => {
      if (event.target === event.currentTarget) {
        event.currentTarget.focus({ preventScroll: true });
        pointerGestureRef.current = true;
      }
    },
    [],
  );

  const onTouchStart = useCallback((event: TouchEvent<HTMLDivElement>) => {
    lastTouchYRef.current = event.touches[0]?.clientY ?? null;
  }, []);

  const onTouchMove = useCallback(
    (event: TouchEvent<HTMLDivElement>) => {
      const currentY = event.touches[0]?.clientY;
      if (currentY !== undefined && lastTouchYRef.current !== null) {
        const intent = resolveTouchFollowScrollIntent(
          lastTouchYRef.current,
          currentY,
        );
        if (intent) {
          recordTransientScrollIntent(intent);
          if (intent === "up") {
            pauseFollowLatest();
          }
        }
        lastTouchYRef.current = currentY;
      }
    },
    [pauseFollowLatest, recordTransientScrollIntent],
  );

  const onTouchEnd = useCallback(() => {
    lastTouchYRef.current = null;
  }, []);

  useEffect(() => {
    const container = scrollRef.current;
    const endPointerGesture = () => {
      pointerGestureRef.current = false;
    };
    const handleKeyDown = (event: KeyboardEvent) => {
      if (isEditableKeyboardTarget(event.target)) {
        return;
      }
      const intent = resolveKeyboardFollowScrollIntent(
        event.key,
        event.shiftKey,
      );
      if (!intent) {
        return;
      }
      recordTransientScrollIntent(intent);
      if (intent === "up" && event.target === container) {
        pauseFollowLatest();
      }
    };
    container?.addEventListener("keydown", handleKeyDown);
    window.addEventListener("pointercancel", endPointerGesture);
    window.addEventListener("pointerup", endPointerGesture);
    return () => {
      container?.removeEventListener("keydown", handleKeyDown);
      window.removeEventListener("pointercancel", endPointerGesture);
      window.removeEventListener("pointerup", endPointerGesture);
      clearTransientScrollIntent();
    };
  }, [
    clearTransientScrollIntent,
    pauseFollowLatest,
    recordTransientScrollIntent,
    scrollRef,
  ]);

  return {
    isUserScrollActive,
    onPointerDown,
    onScroll,
    onTouchEnd,
    onTouchMove,
    onTouchStart,
    onWheel,
  };
}

function isEditableKeyboardTarget(target: EventTarget | null): boolean {
  return (
    target instanceof HTMLInputElement
    || target instanceof HTMLSelectElement
    || target instanceof HTMLTextAreaElement
    || (target instanceof HTMLElement && target.isContentEditable)
  );
}
