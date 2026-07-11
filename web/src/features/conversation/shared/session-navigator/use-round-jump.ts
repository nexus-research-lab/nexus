import { useCallback, useEffect, useRef, useState } from "react";
import type { RefObject } from "react";

import {
  clearConversationRoundNavigationTarget,
  findConversationRoundElement,
  getConversationRoundNavigationTarget,
  isConversationRoundScrollTargetVisible,
  setConversationRoundNavigationTarget,
  type ConversationRoundScrollHandleRef,
} from "../timeline/scroll/round-scroll";
import type { ConversationTimeline } from "../timeline/timeline-model";
import { scrollToTimelineRound } from "./navigation-dom";
import type { SessionNavigationItem } from "./session-navigator-model";

const PENDING_SCROLL_MAX_FRAMES = 30;

interface PendingNavigation {
  navigationRoundId: string;
  scopeKey: string;
  scrollRoundId: string;
}

interface NavigationLoadRequest extends PendingNavigation {
  generation: number;
  id: number;
}

interface NavigationLoadRuntime {
  activeRequest: NavigationLoadRequest | null;
  generation: number;
  queuedTarget: PendingNavigation | null;
  scopeKey: string | null;
}

interface UseRoundJumpOptions {
  activateRound: (roundId: string) => void;
  onLoadRoundWindow?: (roundId: string) => Promise<boolean>;
  onNavigateStart?: () => void;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  timeline: ConversationTimeline;
}

function isRoundLoaded(
  timeline: ConversationTimeline,
  roundId: string,
): boolean {
  return (
    (timeline.message_groups.get(roundId)?.length ?? 0) > 0 ||
    timeline.live_round_ids.includes(roundId)
  );
}

function isSameNavigation(
  left: PendingNavigation,
  right: PendingNavigation,
): boolean {
  return (
    left.scopeKey === right.scopeKey &&
    left.navigationRoundId === right.navigationRoundId &&
    left.scrollRoundId === right.scrollRoundId
  );
}

function isCurrentRequest(
  runtime: NavigationLoadRuntime,
  request: NavigationLoadRequest,
): boolean {
  return (
    runtime.generation === request.generation &&
    runtime.scopeKey === request.scopeKey &&
    runtime.activeRequest?.id === request.id
  );
}

function useNavigationLoadQueue({
  cancelNavigation,
  loadRoundWindow,
  scopeKey,
}: {
  cancelNavigation: (target: PendingNavigation) => void;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  scopeKey: string | null;
}) {
  const loadSequenceRef = useRef(0);
  const runtimeRef = useRef<NavigationLoadRuntime>({
    activeRequest: null,
    generation: 0,
    queuedTarget: null,
    scopeKey: null,
  });
  const drainLoadQueueRef = useRef<() => void>(() => {});
  const latestLoaderRef = useRef(loadRoundWindow);
  latestLoaderRef.current = loadRoundWindow;

  drainLoadQueueRef.current = () => {
    const runtime = runtimeRef.current;
    const target = runtime.queuedTarget;
    const loader = latestLoaderRef.current;
    if (!target || !loader || runtime.activeRequest) {
      return;
    }

    runtime.queuedTarget = null;
    const request: NavigationLoadRequest = {
      ...target,
      generation: runtime.generation,
      id: ++loadSequenceRef.current,
    };
    runtime.activeRequest = request;

    void (async () => {
      try {
        const loaded = await loader(request.scrollRoundId);
        if (!isCurrentRequest(runtimeRef.current, request)) {
          return;
        }
        if (!loaded) {
          cancelNavigation(request);
        }
      } catch (error) {
        if (!isCurrentRequest(runtimeRef.current, request)) {
          return;
        }
        cancelNavigation(request);
        console.warn("加载会话导航轮次失败", {
          error,
          roundId: request.scrollRoundId,
        });
      } finally {
        const currentRuntime = runtimeRef.current;
        if (!isCurrentRequest(currentRuntime, request)) {
          return;
        }
        currentRuntime.activeRequest = null;
        drainLoadQueueRef.current();
      }
    })();
  };

  useEffect(() => {
    const runtime = runtimeRef.current;
    runtime.generation += 1;
    runtime.scopeKey = scopeKey;
    runtime.activeRequest = null;
    runtime.queuedTarget = null;
    return () => {
      runtime.generation += 1;
      runtime.activeRequest = null;
      runtime.queuedTarget = null;
    };
  }, [scopeKey]);

  return useCallback(
    (target: PendingNavigation): boolean => {
      const runtime = runtimeRef.current;
      if (!loadRoundWindow || runtime.scopeKey !== target.scopeKey) {
        return false;
      }
      runtime.queuedTarget = target;
      drainLoadQueueRef.current();
      return true;
    },
    [loadRoundWindow],
  );
}

function usePendingNavigationScroll({
  activateRound,
  cancelNavigation,
  completeNavigation,
  pendingNavigation,
  roundScrollRef,
  scopeKey,
  scrollRef,
  timeline,
}: {
  activateRound: (roundId: string) => void;
  cancelNavigation: (target: PendingNavigation) => void;
  completeNavigation: (target: PendingNavigation) => void;
  pendingNavigation: PendingNavigation | null;
  roundScrollRef?: ConversationRoundScrollHandleRef;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
  timeline: ConversationTimeline;
}): void {
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
    useState<PendingNavigation | null>(null);
  const completeNavigation = useCallback((target: PendingNavigation): void => {
    setPendingNavigation((current) =>
      current && isSameNavigation(current, target) ? null : current,
    );
  }, []);
  const cancelNavigation = useCallback(
    (target: PendingNavigation): void => {
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

  usePendingNavigationScroll({
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
      const target: PendingNavigation = {
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
