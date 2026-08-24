/**
 * INPUT: 跨 optimistic ACK 稳定的节点身份与 TanStack Virtual 动态尺寸测量。
 * OUTPUT: 稳定 item key、未加载索引轮次的占位高度，以及服从共享 scroll owner 的动态测高策略。
 * POS: DM 与 Room 虚拟消息流共用的身份、尺寸变化和单写入者协议。
 */
import { useCallback, useRef, type RefObject } from "react";

const VIRTUAL_ANCHOR_TOLERANCE_PX = 1;
const UNLOADED_ROUND_FALLBACK_HEIGHT_PX = 80;

interface VirtualScrollItem {
  end: number;
}

interface VirtualScrollState {
  getTotalSize?: () => number;
  scrollOffset: number | null;
  scrollRect?: {
    height: number;
  } | null;
}

export interface ConversationVirtualAdjustmentContext {
  bottomScrollActive: boolean;
  followingLatest: boolean;
  navigationActive?: boolean;
  userScrollActive: boolean;
}

export function resolveConversationVirtualPlaceholderHeight(
  isLoaded: boolean,
  estimatedHeight: number | undefined,
): number | undefined {
  if (isLoaded) {
    return undefined;
  }
  if (typeof estimatedHeight === "number" && Number.isFinite(estimatedHeight)) {
    return Math.max(UNLOADED_ROUND_FALLBACK_HEIGHT_PX, estimatedHeight);
  }
  return UNLOADED_ROUND_FALLBACK_HEIGHT_PX;
}

export function useConversationVirtualItemKey(
  nodeIds: readonly string[],
): (index: number) => string {
  const stableNodeIdsRef = useRef(nodeIds);
  if (!areNodeIdsEqual(stableNodeIdsRef.current, nodeIds)) {
    stableNodeIdsRef.current = nodeIds;
  }
  const stableNodeIds = stableNodeIdsRef.current;
  return useCallback(
    (index: number) => stableNodeIds[index],
    [stableNodeIds],
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
  delta: number,
  instance: VirtualScrollState,
  context: ConversationVirtualAdjustmentContext = {
    bottomScrollActive: false,
    followingLatest: false,
    navigationActive: false,
    userScrollActive: false,
  },
): boolean {
  if (context.bottomScrollActive || context.navigationActive) {
    return false;
  }
  if (context.userScrollActive && !context.followingLatest) {
    // READING 的直接操控优先于估高补偿；用户滚动时写回 delta 会让滚轮/
    // 触摸位移与 Virtualizer 互相拉扯。新测量仍进入缓存，只是不反向改写
    // 这一手势 epoch 的 scrollTop。
    return false;
  }
  if (context.followingLatest && delta < 0) {
    // live epoch 的负向测量由 Feed min-height 吸收；若这里同步回写
    // scrollTop，会和浏览器 clamp / bottom animator 形成往返震荡。
    return false;
  }
  const scrollOffset = instance.scrollOffset ?? 0;
  const itemIsAboveViewport = (
    item.end
    <= scrollOffset + VIRTUAL_ANCHOR_TOLERANCE_PX
  );
  if (itemIsAboveViewport) {
    return true;
  }

  const viewportHeight = instance.scrollRect?.height;
  const totalSize = instance.getTotalSize?.();
  if (
    typeof viewportHeight !== "number"
    || typeof totalSize !== "number"
  ) {
    return false;
  }

  // TanStack Virtual 在写入本次 measured size 前调用该策略，因此 totalSize
  // 仍是变更前的真实总高。旧视口若位于底部，就由同一 Virtualizer 写入 delta：
  // 上方 Agent 增长只移动数值 scrollTop，不移动用户眼中的底部内容。
  return (
    totalSize
    <= scrollOffset + viewportHeight + VIRTUAL_ANCHOR_TOLERANCE_PX
  );
}

function areNodeIdsEqual(
  current: readonly string[],
  next: readonly string[],
): boolean {
  return (
    current.length === next.length
    && current.every((nodeId, index) => nodeId === next[index])
  );
}
