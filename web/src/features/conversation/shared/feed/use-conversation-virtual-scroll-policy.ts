/**
 * INPUT: 跨 optimistic ACK 稳定的节点身份与 TanStack Virtual 动态尺寸测量。
 * OUTPUT: 稳定 item key、未加载索引轮次的占位高度、服从共享 scroll owner 的动态测高策略与相对补偿写入。
 * POS: DM 与 Room 虚拟消息流共用的身份、尺寸变化和单写入者协议。
 */
import { useCallback, useRef, type RefObject } from "react";

const VIRTUAL_ANCHOR_TOLERANCE_PX = 1;
const UNLOADED_ROUND_FALLBACK_HEIGHT_PX = 80;

interface VirtualScrollItem {
  end: number;
}

interface VirtualScrollWriter {
  scrollElement: Element | Window | null;
  scrollOffset: number | null;
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
  _delta: number,
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
  const scrollOffset = instance.scrollOffset ?? 0;
  const itemIsAboveViewport = (
    item.end
    <= scrollOffset + VIRTUAL_ANCHOR_TOLERANCE_PX
  );
  if (itemIsAboveViewport) {
    // 完整位于视口上方的高度变化只是内容平移，补偿它不会改变用户的阅读位置；
    // 向上滚动触发的窗口加载正是这种情形（80px 占位换成真实正文），
    // 手势期间若不补偿，视口会被整段新内容推回到刚加载窗口的开头。
    return true;
  }
  if (context.userScrollActive && !context.followingLatest) {
    // 横跨或位于视口下方的变化在直接操控期间不写回，避免与滚轮/触摸位移拉扯。
    return false;
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

/**
 * Virtualizer 的测高补偿以最近一次 scroll 事件记录的 offset 为基准写绝对值；
 * 手势进行中该值可能落后一帧，绝对写入会把用户刚滚过的距离吞掉。补偿写入
 * 改为叠加到实时 scrollTop，其余定位（scrollToIndex/Offset）保持绝对语义。
 */
export function scrollConversationVirtualElement(
  offset: number,
  { adjustments = 0, behavior }: { adjustments?: number; behavior?: ScrollBehavior },
  instance: VirtualScrollWriter,
): void {
  const element = instance.scrollElement;
  if (!element || !("scrollTop" in element)) {
    return;
  }
  if (adjustments !== 0 && behavior !== "smooth" && instance.scrollOffset !== null) {
    element.scrollTop += adjustments;
    return;
  }
  element.scrollTo({ behavior, top: offset + adjustments });
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
