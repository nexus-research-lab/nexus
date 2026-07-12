import { useEffect, type RefObject } from "react";

import {
  type ConversationRoundScrollHandleRef,
} from "../../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../../timeline/timeline-model";
import {
  attemptPendingRoundJumpLanding,
  PendingRoundJumpLandingRuntime,
} from "./pending-round-jump-runtime";
import type { PendingRoundJump } from "./round-jump-model";

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
    const runtime = new PendingRoundJumpLandingRuntime({
      attemptLanding: () => attemptPendingRoundJumpLanding({
        roundScrollHandle: roundScrollRef?.current ?? null,
        scrollElement: scrollRef.current,
        target,
        timeline,
      }),
      onCancel: () => cancelNavigation(target),
      onLand: (navigationRoundId) => {
        activateRound(navigationRoundId);
        completeNavigation(target);
      },
    });
    runtime.start();
    return () => runtime.cancel();
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
