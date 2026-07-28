/**
 * INPUT: canonical 轮次身份与 TanStack Virtual 的动态尺寸测量。
 * OUTPUT: 仅在轮次集合真实变化时更新的 item key，以及不误推可见长回复的锚点策略。
 * POS: DM 与 Room 虚拟消息流共用的身份和尺寸变化滚动协议。
 */
import { useCallback, useRef, type RefObject } from "react";

const VIRTUAL_ANCHOR_TOLERANCE_PX = 1;

interface VirtualScrollItem {
  end: number;
}

interface VirtualScrollState {
  scrollOffset: number | null;
}

export function useConversationVirtualItemKey(
  roundIds: readonly string[],
): (index: number) => string {
  const stableRoundIdsRef = useRef(roundIds);
  if (!areRoundIdsEqual(stableRoundIdsRef.current, roundIds)) {
    stableRoundIdsRef.current = roundIds;
  }
  const stableRoundIds = stableRoundIdsRef.current;
  return useCallback(
    (index: number) => stableRoundIds[index],
    [stableRoundIds],
  );
}

/**
 * 普通 Feed 切换为 Virtualizer 时复用既有视口偏移，不能让新实例先从 0
 * 写回同一个滚动容器。Safari 回弹产生的负值不属于有效初始位置。
 */
export function resolveConversationVirtualInitialOffset(
  scrollElement: Pick<HTMLElement, "scrollTop"> | null,
): number {
  return Math.max(0, scrollElement?.scrollTop ?? 0);
}

export function useConversationVirtualInitialOffset(
  scrollRef: RefObject<HTMLDivElement | null>,
): number {
  const initialOffsetRef = useRef<number | null>(null);
  if (initialOffsetRef.current === null) {
    initialOffsetRef.current = resolveConversationVirtualInitialOffset(
      scrollRef.current,
    );
  }
  return initialOffsetRef.current;
}

export function shouldAdjustConversationVirtualScrollPosition(
  item: VirtualScrollItem,
  _delta: number,
  instance: VirtualScrollState,
): boolean {
  return (
    item.end
    <= (instance.scrollOffset ?? 0) + VIRTUAL_ANCHOR_TOLERANCE_PX
  );
}

function areRoundIdsEqual(
  current: readonly string[],
  next: readonly string[],
): boolean {
  return (
    current.length === next.length
    && current.every((roundId, index) => roundId === next[index])
  );
}
