import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

import { getConversationRoundNavigationTarget } from "./scroll/round-scroll";

const UNLOADED_ROUND_SELECTOR =
  '[data-conversation-round-id][data-conversation-round-loaded="false"]';
const LOAD_ROOT_MARGIN_PX = 180;
const LOAD_RECHECK_DELAY_MS = 80;
const MAX_LOAD_ATTEMPTS = 3;
const RETRY_DELAYS_MS = [250, 1_000, 3_000] as const;

type ScrollDirection = "down" | "none" | "up";

interface UseVisibleRoundWindowLoaderOptions {
  enabled: boolean;
  loadRoundWindow?: (roundId: string) => Promise<boolean>;
  revision: string | number;
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
}

interface VisibleRoundCandidate {
  centerY: number;
  distance: number;
  roundId: string;
}

interface LoadRequest {
  generation: number;
  id: number;
  roundId: string;
}

interface RoundAttempt {
  count: number;
  retryAfter: number;
}

interface LoaderRuntime {
  activeRequest: LoadRequest | null;
  attempts: Map<string, RoundAttempt>;
  completedRoundIds: Set<string>;
  generation: number;
  lastScrollTop: number;
  scopeKey: string | null;
  scrollDirection: ScrollDirection;
}

function resolveVisibleUnloadedRoundId(
  scrollElement: HTMLDivElement,
  excludedRoundIds: Set<string>,
  direction: ScrollDirection,
): string | null {
  const containerRect = scrollElement.getBoundingClientRect();
  const minY = containerRect.top - LOAD_ROOT_MARGIN_PX;
  const maxY = containerRect.bottom + LOAD_ROOT_MARGIN_PX;
  const focusY =
    containerRect.top + Math.min(scrollElement.clientHeight * 0.38, 260);
  const candidates: VisibleRoundCandidate[] = [];

  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(UNLOADED_ROUND_SELECTOR),
  );
  for (const element of elements) {
    const roundId = element.dataset.conversationRoundId?.trim();
    if (!roundId || excludedRoundIds.has(roundId)) {
      continue;
    }

    const rect = element.getBoundingClientRect();
    if (rect.bottom < minY || rect.top > maxY) {
      continue;
    }

    const centerY = rect.top + Math.min(rect.height, 120) / 2;
    candidates.push({
      centerY,
      distance: Math.abs(centerY - focusY),
      roundId,
    });
  }

  if (candidates.length === 0) {
    return null;
  }

  if (direction === "down") {
    const nextBelow = candidates
      .filter((candidate) => candidate.centerY >= focusY)
      .reduce<VisibleRoundCandidate | null>(
        (best, candidate) =>
          !best || candidate.centerY < best.centerY ? candidate : best,
        null,
      );
    if (nextBelow) {
      return nextBelow.roundId;
    }
  }

  if (direction === "up") {
    const nextAbove = candidates
      .filter((candidate) => candidate.centerY <= focusY)
      .reduce<VisibleRoundCandidate | null>(
        (best, candidate) =>
          !best || candidate.centerY > best.centerY ? candidate : best,
        null,
      );
    if (nextAbove) {
      return nextAbove.roundId;
    }
  }

  return candidates.reduce((best, candidate) =>
    candidate.distance < best.distance ? candidate : best,
  ).roundId;
}

function buildExcludedRoundIds(
  runtime: LoaderRuntime,
  now: number,
): Set<string> {
  const excluded = new Set(runtime.completedRoundIds);
  if (runtime.activeRequest) {
    excluded.add(runtime.activeRequest.roundId);
  }
  for (const [roundId, attempt] of runtime.attempts) {
    if (attempt.count >= MAX_LOAD_ATTEMPTS || attempt.retryAfter > now) {
      excluded.add(roundId);
    }
  }
  return excluded;
}

function isCurrentRequest(
  runtime: LoaderRuntime,
  request: LoadRequest,
): boolean {
  return (
    runtime.generation === request.generation &&
    runtime.activeRequest?.id === request.id
  );
}

export function useVisibleRoundWindowLoader({
  enabled,
  loadRoundWindow,
  revision,
  scopeKey,
  scrollRef,
}: UseVisibleRoundWindowLoaderOptions) {
  const frameRef = useRef<number | null>(null);
  const requestSequenceRef = useRef(0);
  const runCheckRef = useRef<() => void>(() => {});
  const runtimeRef = useRef<LoaderRuntime>({
    activeRequest: null,
    attempts: new Map(),
    completedRoundIds: new Set(),
    generation: 0,
    lastScrollTop: 0,
    scopeKey: null,
    scrollDirection: "none",
  });
  const timeoutRef = useRef<number | null>(null);
  const latestOptionsRef = useRef({ enabled, loadRoundWindow, scopeKey });
  latestOptionsRef.current = { enabled, loadRoundWindow, scopeKey };

  const scheduleCheck = useCallback(() => {
    if (frameRef.current !== null) {
      return;
    }
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null;
      runCheckRef.current();
    });
  }, []);

  const scheduleRetry = useCallback(
    (delay: number) => {
      if (timeoutRef.current !== null) {
        window.clearTimeout(timeoutRef.current);
      }
      timeoutRef.current = window.setTimeout(() => {
        timeoutRef.current = null;
        scheduleCheck();
      }, delay);
    },
    [scheduleCheck],
  );

  runCheckRef.current = () => {
    const latest = latestOptionsRef.current;
    const runtime = runtimeRef.current;
    if (
      !latest.enabled ||
      !latest.loadRoundWindow ||
      runtime.activeRequest ||
      runtime.scopeKey !== latest.scopeKey
    ) {
      return;
    }

    const scrollElement = scrollRef.current;
    if (!scrollElement || getConversationRoundNavigationTarget(scrollElement)) {
      return;
    }

    const roundId = resolveVisibleUnloadedRoundId(
      scrollElement,
      buildExcludedRoundIds(runtime, Date.now()),
      runtime.scrollDirection,
    );
    if (!roundId) {
      return;
    }

    const request: LoadRequest = {
      generation: runtime.generation,
      id: ++requestSequenceRef.current,
      roundId,
    };
    runtime.activeRequest = request;

    void (async () => {
      let nextCheckDelay: number | null = LOAD_RECHECK_DELAY_MS;
      try {
        const didLoad = await latest.loadRoundWindow!(roundId);
        if (!isCurrentRequest(runtimeRef.current, request)) {
          return;
        }
        if (didLoad) {
          runtime.completedRoundIds.add(roundId);
          runtime.attempts.delete(roundId);
        } else {
          const previousCount = runtime.attempts.get(roundId)?.count ?? 0;
          const count = previousCount + 1;
          runtime.attempts.set(roundId, {
            count,
            retryAfter: Date.now() + RETRY_DELAYS_MS[count - 1],
          });
          nextCheckDelay =
            count < MAX_LOAD_ATTEMPTS ? RETRY_DELAYS_MS[count - 1] : null;
        }
      } catch (error) {
        if (!isCurrentRequest(runtimeRef.current, request)) {
          return;
        }
        const previousCount = runtime.attempts.get(roundId)?.count ?? 0;
        const count = previousCount + 1;
        runtime.attempts.set(roundId, {
          count,
          retryAfter: Date.now() + RETRY_DELAYS_MS[count - 1],
        });
        nextCheckDelay =
          count < MAX_LOAD_ATTEMPTS ? RETRY_DELAYS_MS[count - 1] : null;
        console.warn("加载可见对话轮次失败", { error, roundId });
      } finally {
        const currentRuntime = runtimeRef.current;
        if (!isCurrentRequest(currentRuntime, request)) {
          return;
        }
        currentRuntime.activeRequest = null;
        if (nextCheckDelay !== null) {
          scheduleRetry(nextCheckDelay);
        }
      }
    })();
  };

  useEffect(() => {
    const runtime = runtimeRef.current;
    runtime.generation += 1;
    runtime.scopeKey = scopeKey;
    runtime.activeRequest = null;
    runtime.attempts.clear();
    runtime.completedRoundIds.clear();
    runtime.lastScrollTop = scrollRef.current?.scrollTop ?? 0;
    runtime.scrollDirection = "none";
    scheduleCheck();
  }, [scheduleCheck, scopeKey, scrollRef]);

  useEffect(() => {
    runtimeRef.current.attempts.clear();
    scheduleCheck();
  }, [revision, scheduleCheck]);

  useEffect(() => {
    scheduleCheck();
  }, [enabled, loadRoundWindow, scheduleCheck]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }

    const runtime = runtimeRef.current;
    runtime.lastScrollTop = scrollElement.scrollTop;
    const handleScroll = () => {
      const currentRuntime = runtimeRef.current;
      const nextScrollTop = scrollElement.scrollTop;
      if (nextScrollTop > currentRuntime.lastScrollTop) {
        currentRuntime.scrollDirection = "down";
      } else if (nextScrollTop < currentRuntime.lastScrollTop) {
        currentRuntime.scrollDirection = "up";
      }
      currentRuntime.lastScrollTop = nextScrollTop;
      scheduleCheck();
    };
    scheduleCheck();
    scrollElement.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("resize", handleScroll);
    return () => {
      scrollElement.removeEventListener("scroll", handleScroll);
      window.removeEventListener("resize", handleScroll);
      runtime.generation += 1;
      runtime.activeRequest = null;
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current);
        frameRef.current = null;
      }
      if (timeoutRef.current !== null) {
        window.clearTimeout(timeoutRef.current);
        timeoutRef.current = null;
      }
    };
  }, [scheduleCheck, scrollRef]);
}
