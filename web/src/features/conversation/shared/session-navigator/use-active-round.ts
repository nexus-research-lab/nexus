import { useCallback, useEffect, useState } from "react";
import type { RefObject } from "react";

import {
  clearConversationRoundNavigationTarget,
  getConversationRoundNavigationTarget,
} from "../timeline/scroll/round-scroll";
import { resolveVisibleRoundId } from "./navigation-dom";

const SCROLL_NAVIGATION_KEYS = new Set([
  "ArrowDown",
  "ArrowUp",
  "End",
  "Home",
  "PageDown",
  "PageUp",
  " ",
]);

interface ActiveRoundSelection {
  roundId: string;
  scopeKey: string;
}

interface UseActiveRoundOptions {
  roundIds: string[];
  scopeKey: string | null;
  scrollRef: RefObject<HTMLDivElement | null>;
}

export function useActiveRound({
  roundIds,
  scopeKey,
  scrollRef,
}: UseActiveRoundOptions) {
  const [selection, setSelection] = useState<ActiveRoundSelection | null>(null);
  const selectedRoundId =
    selection?.scopeKey === scopeKey && roundIds.includes(selection.roundId)
      ? selection.roundId
      : null;
  const activeRoundId = selectedRoundId ?? roundIds[roundIds.length - 1] ?? null;

  const activateRound = useCallback(
    (roundId: string): void => {
      if (scopeKey) {
        setSelection((current) =>
          current?.scopeKey === scopeKey && current.roundId === roundId
            ? current
            : { roundId, scopeKey },
        );
      }
    },
    [scopeKey],
  );

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement || !scopeKey || roundIds.length === 0) {
      return;
    }
    let frame = 0;
    const roundIdSet = new Set(roundIds);
    const syncActiveRound = (): void => {
      frame = 0;
      const navigationTarget = getConversationRoundNavigationTarget(
        scrollElement,
      );
      if (navigationTarget && !roundIdSet.has(navigationTarget)) {
        clearConversationRoundNavigationTarget(
          scrollElement,
          navigationTarget,
        );
      }
      // 导航目标只是进行中的滚动事务，不是已经可见的 active round。
      // 真正落点后由 pending jump 显式提交，加载/测量期间保留当前选择。
      if (navigationTarget && roundIdSet.has(navigationTarget)) {
        return;
      }
      const nextRoundId = resolveVisibleRoundId(
        scrollElement,
        roundIds,
        roundIdSet,
      );
      if (nextRoundId) {
        activateRound(nextRoundId);
      }
    };
    const scheduleSync = (): void => {
      if (!frame) {
        frame = window.requestAnimationFrame(syncActiveRound);
      }
    };

    syncActiveRound();
    scrollElement.addEventListener("scroll", scheduleSync, { passive: true });
    window.addEventListener("resize", scheduleSync);
    return () => {
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
      scrollElement.removeEventListener("scroll", scheduleSync);
      window.removeEventListener("resize", scheduleSync);
    };
  }, [activateRound, roundIds, scopeKey, scrollRef]);

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const clearNavigationTarget = (): void => {
      clearConversationRoundNavigationTarget(scrollElement);
    };
    const handleKeyDown = (event: KeyboardEvent): void => {
      if (SCROLL_NAVIGATION_KEYS.has(event.key)) {
        clearNavigationTarget();
      }
    };

    scrollElement.addEventListener("wheel", clearNavigationTarget, {
      passive: true,
    });
    scrollElement.addEventListener("touchstart", clearNavigationTarget, {
      passive: true,
    });
    scrollElement.addEventListener("pointerdown", clearNavigationTarget, {
      passive: true,
    });
    window.addEventListener("keydown", handleKeyDown);
    return () => {
      clearNavigationTarget();
      scrollElement.removeEventListener("wheel", clearNavigationTarget);
      scrollElement.removeEventListener("touchstart", clearNavigationTarget);
      scrollElement.removeEventListener("pointerdown", clearNavigationTarget);
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [scopeKey, scrollRef]);

  return { activeRoundId, activateRound };
}
