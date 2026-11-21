/**
 * INPUT: 跨 optimistic ACK 稳定的节点身份与 TanStack Virtual 动态尺寸测量。
 * OUTPUT: 稳定 item key、连续窗口拓扑变化时的阅读锚点、服从共享 scroll owner 的动态测高策略与相对补偿写入。
 * POS: DM 与 Room 虚拟消息流共用的身份、尺寸变化和单写入者协议。
 */
import { useCallback, useLayoutEffect, useRef, type RefObject } from "react";

import {
  scrollConversationElementTo,
  shiftConversationScrollTop,
} from "../timeline/scroll/scroll-controller";

const VIRTUAL_ANCHOR_TOLERANCE_PX = 1;

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

/**
 * TanStack 在两次 scroll 事件之间把测高位移累加后交给 scrollToFn。补偿叠加到实时
 * scrollTop 时只能写入本次测量的位移，这里把它留给紧随其后的那次写入。
 */
const pendingSizeCompensation = new WeakMap<object, number>();

export function shouldAdjustConversationVirtualScrollPosition(
  item: VirtualScrollItem,
  delta: number,
  instance: VirtualScrollState,
  context?: ConversationVirtualAdjustmentContext,
): boolean {
  const compensate = shouldCompensateVirtualSizeChange(item, instance, context);
  if (compensate) {
    pendingSizeCompensation.set(instance, delta);
  }
  return compensate;
}

function shouldCompensateVirtualSizeChange(
  item: VirtualScrollItem,
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
 * 改为把本次测量的位移叠加到实时 scrollTop，其余定位（scrollToIndex/Offset）
 * 保持绝对语义。
 */
export function scrollConversationVirtualElement(
  offset: number,
  { adjustments = 0, behavior }: { adjustments?: number; behavior?: ScrollBehavior },
  instance: VirtualScrollWriter,
): void {
  const sizeDelta = pendingSizeCompensation.get(instance);
  pendingSizeCompensation.delete(instance);
  const element = instance.scrollElement;
  if (!element || !("scrollTop" in element)) {
    return;
  }
  if (sizeDelta !== undefined && behavior !== "smooth") {
    shiftConversationScrollTop(element, "virtualizer", sizeDelta);
    return;
  }
  scrollConversationElementTo(element, "virtualizer", {
    behavior,
    top: offset + adjustments,
  });
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

interface VirtualTopologyAnchor {
  index: number;
  key: string;
  start: number;
}

interface VirtualTopologyInstance {
  measurementsCache: ReadonlyArray<{ end: number; start: number }>;
  scrollElement: Element | Window | null;
  scrollOffset: number | null;
}

/**
 * 连续窗口在阅读位置之前增删 round（历史前插、起点淘汰、回到最新）时，
 * Virtualizer 只会按新下标重新排布，不会移动 scrollTop。这里在每次提交后
 * 记录首个可见项的身份与起点；该项下标变化即说明它之前的拓扑变了，按起点
 * 位移补偿实时 scrollTop，让同一 round 留在原位。项自身或上方的测高变化
 * 不改变下标，继续由 Virtualizer 的尺寸补偿负责，两者不会重复写入。
 */
export function useConversationVirtualTopologyAnchor(
  instance: VirtualTopologyInstance,
  nodeIds: readonly string[],
  shouldHold: () => boolean,
): void {
  const anchorRef = useRef<VirtualTopologyAnchor | null>(null);

  useLayoutEffect(() => {
    const scrollElement = instance.scrollElement;
    const element = scrollElement && "scrollTop" in scrollElement
      ? scrollElement
      : null;
    const measurements = instance.measurementsCache;
    const anchor = anchorRef.current;
    if (
      anchor
      && element
      && nodeIds[anchor.index] !== anchor.key
      && shouldHold()
    ) {
      const nextIndex = nodeIds.indexOf(anchor.key);
      const delta = nextIndex < 0
        ? 0
        : (measurements[nextIndex]?.start ?? anchor.start) - anchor.start;
      if (delta !== 0) {
        shiftConversationScrollTop(element, "virtualizer", delta);
      }
    }

    const scrollOffset = element?.scrollTop ?? instance.scrollOffset ?? 0;
    const index = findFirstVisibleMeasurement(measurements, scrollOffset);
    anchorRef.current = index < 0 || index >= nodeIds.length
      ? null
      : { index, key: nodeIds[index], start: measurements[index].start };
  });
}

function findFirstVisibleMeasurement(
  measurements: ReadonlyArray<{ end: number }>,
  scrollOffset: number,
): number {
  let low = 0;
  let high = measurements.length - 1;
  let found = -1;
  while (low <= high) {
    const middle = (low + high) >> 1;
    if (measurements[middle].end > scrollOffset) {
      found = middle;
      high = middle - 1;
    } else {
      low = middle + 1;
    }
  }
  return found;
}
