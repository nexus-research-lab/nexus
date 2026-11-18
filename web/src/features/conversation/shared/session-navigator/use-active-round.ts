import { useCallback, useEffect, useRef, useState } from "react";
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

  // 流式帧每次都会生成新的 roundIds 数组；若把它作为订阅依赖，每帧都会重建监听并在
  // 提交阶段同步读取 scrollTop/scrollHeight，强制一次布局。这里只在作用域变化时订阅，
  // 轮次变化改为排入下一帧，与浏览器自身布局合并。
  const roundsRef = useRef({ ids: roundIds, set: new Set(roundIds) });
  const scheduleSyncRef = useRef<(() => void) | null>(null);
  const hasRounds = roundIds.length > 0;

  useEffect(() => {
    const scrollElement = scrollRef.current;
    if (!scrollElement || !scopeKey || !hasRounds) {
      return;
    }
    let frame = 0;
    const syncActiveRound = (): void => {
      frame = 0;
      const { ids, set } = roundsRef.current;
      if (ids.length === 0) {
        return;
      }
      const navigationTarget = getConversationRoundNavigationTarget(
        scrollElement,
      );
      if (navigationTarget && !set.has(navigationTarget)) {
        clearConversationRoundNavigationTarget(
          scrollElement,
          navigationTarget,
        );
      }
      // 导航目标只是进行中的滚动事务，不是已经可见的 active round。
      // 真正落点后由 pending jump 显式提交，加载/测量期间保留当前选择。
      if (navigationTarget && set.has(navigationTarget)) {
        return;
      }
      const nextRoundId = resolveVisibleRoundId(scrollElement, ids, set);
      if (nextRoundId) {
        activateRound(nextRoundId);
      }
    };
    const scheduleSync = (): void => {
      if (!frame) {
        frame = window.requestAnimationFrame(syncActiveRound);
      }
    };

    scheduleSyncRef.current = scheduleSync;
    syncActiveRound();
    scrollElement.addEventListener("scroll", scheduleSync, { passive: true });
    window.addEventListener("resize", scheduleSync);
    return () => {
      scheduleSyncRef.current = null;
      if (frame) {
        window.cancelAnimationFrame(frame);
      }
      scrollElement.removeEventListener("scroll", scheduleSync);
      window.removeEventListener("resize", scheduleSync);
    };
  }, [activateRound, hasRounds, scopeKey, scrollRef]);

  useEffect(() => {
    if (roundsRef.current.ids === roundIds) {
      return;
    }
    roundsRef.current = { ids: roundIds, set: new Set(roundIds) };
    scheduleSyncRef.current?.();
  }, [roundIds]);

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
