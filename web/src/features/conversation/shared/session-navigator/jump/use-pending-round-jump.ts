import { useEffect, type RefObject } from "react";

import {
  findConversationRoundElement,
  getConversationRoundNavigationTarget,
  isConversationRoundScrollTargetVisible,
  type ConversationRoundScrollHandleRef,
} from "../../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../../timeline/timeline-model";
import { scrollToTimelineRound } from "../navigation-dom";
import {
  isRoundLoaded,
  type PendingRoundJump,
} from "./round-jump-model";

const PENDING_SCROLL_MAX_FRAMES = 30;

interface UsePendingRoundJumpOptions {
  activateRound: (roundId: string) => void;
  cancelNavigation: (target: PendingRoundJump) => void;
  completeNavigation: (target: PendingRoundJump) => void;
  pendingNavigation: PendingRoundJump | null;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  timeline: ConversationTimeline;
}

export function usePendingRoundJump({
  activateRound,
  cancelNavigation,
  completeNavigation,
  pendingNavigation,
  roundScrollRef,
  scopeKey,
  scrollRef,
  timeline,
}: UsePendingRoundJumpOptions): void {
  useEffect(() => {
    const target = pendingNavigation;
    if (!target || target.scopeKey !== scopeKey) {
      return;
    }
    let frame = 0;
    let attemptCount = 0;

    const scheduleRetry = (): void => {
      if (attemptCount >= PENDING_SCROLL_MAX_FRAMES) {
        cancelNavigation(target);
        return;
      }
      attemptCount += 1;
      frame = window.requestAnimationFrame(tryScroll);
    };

    const tryScroll = (): void => {
      frame = 0;
      const loaded = isRoundLoaded(timeline, target.scrollRoundId);
      const didScroll = scrollToTimelineRound(
        scrollRef.current,
        roundScrollRef?.current ?? null,
        target.scrollRoundId,
        {
          align: "focus",
          behavior: loaded ? "auto" : "smooth",
        },
      );
      if (!didScroll) {
        scheduleRetry();
        return;
      }
      if (!loaded) {
        return;
      }

      const scrollElement = scrollRef.current;
      const roundElement = scrollElement
        ? findConversationRoundElement(scrollElement, target.scrollRoundId)
        : null;
      if (
        !scrollElement ||
        !roundElement ||
        !isConversationRoundScrollTargetVisible(scrollElement, roundElement)
      ) {
        scheduleRetry();
        return;
      }
      activateRound(
        getConversationRoundNavigationTarget(scrollElement) ??
          target.navigationRoundId,
      );
      completeNavigation(target);
    };

    frame = window.requestAnimationFrame(tryScroll);
    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
    };
  }, [
    activateRound,
    cancelNavigation,
    completeNavigation,
    pendingNavigation,
    roundScrollRef,
    scopeKey,
    scrollRef,
    timeline,
  ]);
}
