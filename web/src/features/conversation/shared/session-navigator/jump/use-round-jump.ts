import { useCallback, useState, type RefObject } from "react";

import {
  clearConversationRoundNavigationTarget,
  setConversationRoundNavigationTarget,
  type ConversationRoundScrollHandleRef,
} from "../../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../../timeline/timeline-model";
import { scrollToTimelineRound } from "../navigation-dom";
import type { SessionNavigationItem } from "../session-navigator-model";
import {
  isRoundLoaded,
  isSameRoundJump,
  type PendingRoundJump,
} from "./round-jump-model";
import { useNavigationLoadQueue } from "./use-navigation-load-queue";
import { usePendingRoundJump } from "./use-pending-round-jump";

interface UseRoundJumpOptions {
  activateRound: (roundId: string) => void;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  timeline: ConversationTimeline;
}

export function useRoundJump({
  activateRound,
  onLoadRoundWindow,
  onNavigateStart,
  roundScrollRef,
  scopeKey,
  scrollRef,
  timeline,
}: UseRoundJumpOptions) {
  const [pendingNavigation, setPendingNavigation] =
    useState<PendingRoundJump | null>(null);
  const completeNavigation = useCallback((target: PendingRoundJump): void => {
    setPendingNavigation((current) => (
      current && isSameRoundJump(current, target) ? null : current
    ));
  }, []);
  const cancelNavigation = useCallback(
    (target: PendingRoundJump): void => {
      completeNavigation(target);
      const scrollElement = scrollRef.current;
      if (scrollElement) {
        clearConversationRoundNavigationTarget(
          scrollElement,
          target.navigationRoundId,
        );
      }
    },
    [completeNavigation, scrollRef],
  );
  const enqueueNavigationLoad = useNavigationLoadQueue({
    cancelNavigation,
    loadRoundWindow: onLoadRoundWindow,
    scopeKey,
  });

  usePendingRoundJump({
    activateRound,
    cancelNavigation,
    completeNavigation,
    pendingNavigation,
    roundScrollRef,
    scopeKey,
    scrollRef,
    timeline,
  });

  const jumpToRound = useCallback(
    (item: SessionNavigationItem): void => {
      const scrollElement = scrollRef.current;
      if (!scrollElement || !scopeKey) {
        return;
      }
      const target: PendingRoundJump = {
        navigationRoundId: item.roundId,
        scopeKey,
        scrollRoundId: item.inputRoundId,
      };
      const loaded = isRoundLoaded(timeline, target.scrollRoundId);

      onNavigateStart?.();
      setConversationRoundNavigationTarget(
        scrollElement,
        target.navigationRoundId,
      );
      activateRound(target.navigationRoundId);
      setPendingNavigation(target);
      scrollToTimelineRound(
        scrollElement,
        roundScrollRef?.current ?? null,
        target.scrollRoundId,
        { align: "focus", behavior: "smooth" },
      );

      if (!loaded && !enqueueNavigationLoad(target)) {
        cancelNavigation(target);
      }
    },
    [
      activateRound,
      cancelNavigation,
      enqueueNavigationLoad,
      onNavigateStart,
      roundScrollRef,
      scopeKey,
      scrollRef,
      timeline,
    ],
  );

  return { jumpToRound };
}
