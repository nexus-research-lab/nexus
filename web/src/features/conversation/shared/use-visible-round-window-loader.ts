import { useCallback, useEffect, useRef } from "react";
import type { RefObject } from "react";

import { getConversationRoundNavigationTarget } from "./conversation-round-scroll";

const UNLOADED_ROUND_SELECTOR =
  '[data-conversation-round-id][data-conversation-round-loaded="false"]';
const LOAD_ROOT_MARGIN_PX = 180;
const LOAD_RECHECK_DELAY_MS = 80;

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

function resolveVisibleUnloadedRoundId(
  scrollElement: HTMLDivElement,
  attemptedRoundIds: Set<string>,
  direction: ScrollDirection,
): string | null {
  const containerRect = scrollElement.getBoundingClientRect();
  const minY = containerRect.top - LOAD_ROOT_MARGIN_PX;
  const maxY = containerRect.bottom + LOAD_ROOT_MARGIN_PX;
  const focusY = containerRect.top + Math.min(scrollElement.clientHeight * 0.38, 260);
  const candidates: VisibleRoundCandidate[] = [];

  const elements = Array.from(
    scrollElement.querySelectorAll<HTMLElement>(UNLOADED_ROUND_SELECTOR),
  );
  for (const element of elements) {
    const roundId = element.dataset.conversationRoundId?.trim();
    if (!roundId || attemptedRoundIds.has(roundId)) {
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

export function useVisibleRoundWindowLoader({
  enabled,
  loadRoundWindow,
  revision,
  scopeKey,
  scrollRef,
}: UseVisibleRoundWindowLoaderOptions) {
  const attemptedRoundIdsRef = useRef(new Set<string>());
  const frameRef = useRef<number | null>(null);
  const isLoadingRef = useRef(false);
  const latestOptionsRef = useRef({ enabled, loadRoundWindow });
  const lastScrollTopRef = useRef(0);
  const runCheckRef = useRef<() => void>(() => {});
  const scrollDirectionRef = useRef<ScrollDirection>("none");
  const timeoutRef = useRef<number | null>(null);

  useEffect(() => {
    latestOptionsRef.current = { enabled, loadRoundWindow };
  }, [enabled, loadRoundWindow]);

  const scheduleCheck = useCallback(() => {
    if (frameRef.current !== null) {
      return;
    }
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null;
      runCheckRef.current();
    });
  }, []);

  useEffect(() => {
    runCheckRef.current = () => {
      const { enabled: latestEnabled, loadRoundWindow: latestLoadRoundWindow } =
        latestOptionsRef.current;
      if (!latestEnabled || !latestLoadRoundWindow || isLoadingRef.current) {
        return;
      }

      const scrollElement = scrollRef.current;
      if (!scrollElement) {
        return;
      }
      if (getConversationRoundNavigationTarget(scrollElement)) {
        return;
      }

      const roundId = resolveVisibleUnloadedRoundId(
        scrollElement,
        attemptedRoundIdsRef.current,
        scrollDirectionRef.current,
      );
      if (!roundId) {
        return;
      }

      attemptedRoundIdsRef.current.add(roundId);
      isLoadingRef.current = true;
      void latestLoadRoundWindow(roundId).finally(() => {
        isLoadingRef.current = false;
        if (timeoutRef.current !== null) {
          window.clearTimeout(timeoutRef.current);
        }
        timeoutRef.current = window.setTimeout(() => {
          timeoutRef.current = null;
          scheduleCheck();
        }, LOAD_RECHECK_DELAY_MS);
      });
    };
  }, [scheduleCheck, scrollRef]);

  useEffect(() => {
    attemptedRoundIdsRef.current = new Set<string>();
    isLoadingRef.current = false;
    lastScrollTopRef.current = scrollRef.current?.scrollTop ?? 0;
    scrollDirectionRef.current = "none";
    scheduleCheck();
  }, [scheduleCheck, scopeKey, scrollRef]);

  useEffect(() => {
    scheduleCheck();
  }, [revision, scheduleCheck]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }

    lastScrollTopRef.current = scrollElement.scrollTop;
    const handleScroll = () => {
      const nextScrollTop = scrollElement.scrollTop;
      if (nextScrollTop > lastScrollTopRef.current) {
        scrollDirectionRef.current = "down";
      } else if (nextScrollTop < lastScrollTopRef.current) {
        scrollDirectionRef.current = "up";
      }
      lastScrollTopRef.current = nextScrollTop;
      scheduleCheck();
    };
    scheduleCheck();
    scrollElement.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("resize", handleScroll);
    return () => {
      scrollElement.removeEventListener("scroll", handleScroll);
      window.removeEventListener("resize", handleScroll);
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
