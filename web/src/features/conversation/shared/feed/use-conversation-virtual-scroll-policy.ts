/**
 * INPUT: 跨 optimistic ACK 稳定的节点身份与 TanStack Virtual 动态尺寸测量。
 * OUTPUT: 稳定 item key、连续窗口拓扑变化时的阅读锚点、服从共享 scroll owner 的动态测高策略与相对补偿写入。
 * POS: DM 与 Room 虚拟消息流共用的身份、尺寸变化和单写入者协议。
 */
import type { Virtualizer } from "@tanstack/react-virtual";
import { useCallback, useLayoutEffect, useRef, type RefObject } from "react";

import {
  scrollConversationElementTo,
  shiftConversationScrollTop,
} from "../timeline/scroll/scroll-controller";

const VIRTUAL_ANCHOR_TOLERANCE_PX = 1;

interface VirtualScrollItem {
  end: number;
  index?: number;
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
  /**
   * 窗口拓扑变化（历史前插、头部淘汰）提交期间阅读锚点的新下标。此时 scrollTop
   * 仍是旧拓扑下的位置，“是否在视口上方”必须改按锚点判断：只补偿锚点之前的项。
   */
  topologyAnchorIndex?: number | null;
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

export interface ConversationVirtualReadingAnchor {
  index: number;
  offsetTop: number;
}

export interface ConversationVirtualHandoff {
  initialOffset: number;
  readingAnchor: ConversationVirtualReadingAnchor | null;
}

/**
 * 静态 Feed 的 scrollTop 按真实高度累计，Virtualizer 的首个渲染区间却按估高计算；
 * 直接继承 scrollTop 可能让正在阅读的轮次落在区间之外。阅读中切换时改用该轮次的
 * 估高起点作为初始偏移，保证它在首批行内，再由拓扑锚点按真实 DOM 位置恢复。
 */
export function resolveConversationVirtualHandoff(
  scrollElement: HTMLElement | null,
  estimateSize: (index: number) => number,
  holdReading: boolean,
): ConversationVirtualHandoff {
  const readingAnchor = holdReading && scrollElement
    ? captureStaticReadingAnchor(scrollElement)
    : null;
  if (!readingAnchor) {
    return {
      initialOffset: resolveConversationVirtualInitialOffset(scrollElement),
      readingAnchor: null,
    };
  }
  let start = 0;
  for (let index = 0; index < readingAnchor.index; index += 1) {
    start += estimateSize(index);
  }
  const scrolledInto = Math.min(
    Math.max(0, -readingAnchor.offsetTop),
    Math.max(0, estimateSize(readingAnchor.index) - 1),
  );
  return { initialOffset: start + scrolledInto, readingAnchor };
}

export function useConversationVirtualHandoff(
  scrollRef: RefObject<HTMLDivElement | null>,
  estimateSize: (index: number) => number,
  shouldHoldReading: () => boolean,
): ConversationVirtualHandoff {
  const handoffRef = useRef<ConversationVirtualHandoff | null>(null);
  if (handoffRef.current === null) {
    // 首次 render 时静态 Feed 仍在 DOM 中，这是读取切换前阅读位置的唯一时机。
    handoffRef.current = resolveConversationVirtualHandoff(
      scrollRef.current,
      estimateSize,
      shouldHoldReading(),
    );
  }
  return handoffRef.current;
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
  if (
    typeof context.topologyAnchorIndex === "number"
    && typeof item.index === "number"
  ) {
    return item.index < context.topologyAnchorIndex;
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

/**
 * 用户滚动期间 TanStack 不在 ref 阶段测量新挂载的行，而是等 ResizeObserver：
 * 那时 scrollTop 已补偿，行位置却要到下一帧才重排，中间会画出一帧错位。
 * 这里在同一次 layout commit 内测完新行，让补偿与重排落在同一帧。
 */
function measureNewlyMountedRows<
  TScrollElement extends Element | Window,
  TItemElement extends Element,
>(
  instance: Virtualizer<TScrollElement, TItemElement>,
  measured: WeakSet<Element>,
): void {
  for (const node of instance.elementsCache.values()) {
    if (measured.has(node) || !node.isConnected) {
      continue;
    }
    measured.add(node);
    instance.resizeItem(
      instance.indexFromElement(node),
      instance.options.measureElement(node, undefined, instance),
    );
  }
}

/**
 * 连续窗口在阅读位置之前增删 round（历史前插、起点淘汰、回到最新）时，
 * Virtualizer 只会按新下标重新排布，不会移动 scrollTop。这里在每次提交后
 * 记录首个可见项的身份与起点；该项下标变化即说明它之前的拓扑变了，按起点
 * 位移补偿实时 scrollTop，让同一 round 留在原位。项自身或上方的测高变化
 * 不改变下标，继续由 Virtualizer 的尺寸补偿负责，两者不会重复写入。
 */
export function useConversationVirtualTopologyAnchor<
  TScrollElement extends Element | Window,
  TItemElement extends Element,
>(
  instance: Virtualizer<TScrollElement, TItemElement>,
  nodeIds: readonly string[],
  shouldHold: () => boolean,
  handoffAnchor: ConversationVirtualReadingAnchor | null = null,
): () => number | null {
  const anchorRef = useRef<VirtualTopologyAnchor | null>(null);
  const pendingIndexRef = useRef<number | null>(null);
  const measuredRowsRef = useRef(new WeakSet<Element>());
  // Virtualizer 拿到视口尺寸后才渲染行：首批行挂载并恢复切换前的阅读位置之前不记录锚点。
  const settledRef = useRef(false);

  // 非滚动期间新挂载的行在 commit 的 ref 阶段就会测高，早于下面的 layout effect；
  // 所以锚点的新下标必须在 render 阶段就可读，供测高补偿按锚点而非旧视口判断。
  pendingIndexRef.current = shouldHold()
    ? resolvePendingAnchorIndex(anchorRef.current, nodeIds, settledRef.current)
    : null;

  useLayoutEffect(() => {
    const scrollElement = instance.scrollElement;
    const element = scrollElement instanceof Element ? scrollElement : null;
    const measurements = instance.measurementsCache;
    const held = anchorRef.current;
    const nextIndex = pendingIndexRef.current;
    if (held && element && nextIndex !== null) {
      const delta = (measurements[nextIndex]?.start ?? held.start) - held.start;
      if (delta !== 0) {
        shiftConversationScrollTop(element, "virtualizer", delta);
      }
    }
    // 新行的测高补偿仍要按锚点下标判断，所以在清除 pending 下标之前测量。
    measureNewlyMountedRows(instance, measuredRowsRef.current);
    pendingIndexRef.current = null;
    if (!settledRef.current) {
      if (instance.elementsCache.size === 0) {
        return;
      }
      settledRef.current = true;
      if (element && handoffAnchor) {
        restoreStaticReadingAnchor(instance, element, handoffAnchor);
      }
    }

    const scrollOffset = element?.scrollTop ?? instance.scrollOffset ?? 0;
    const index = findFirstVisibleMeasurement(measurements, scrollOffset);
    anchorRef.current = index < 0 || index >= nodeIds.length
      ? null
      : { index, key: nodeIds[index], start: measurements[index].start };
  });

  return useCallback(() => pendingIndexRef.current, []);
}

function resolvePendingAnchorIndex(
  anchor: VirtualTopologyAnchor | null,
  nodeIds: readonly string[],
  settled: boolean,
): number | null {
  if (!settled) {
    // 首批行在画布内按真实高度流式排布，首测只校正估高、不移动任何内容；
    // 阅读位置由 restoreStaticReadingAnchor 一次恢复。
    return 0;
  }
  if (!anchor || nodeIds[anchor.index] === anchor.key) {
    return null;
  }
  const index = nodeIds.indexOf(anchor.key);
  return index < 0 ? null : index;
}

function captureStaticReadingAnchor(
  scrollElement: HTMLElement,
): ConversationVirtualReadingAnchor | null {
  const viewportTop = scrollElement.getBoundingClientRect().top;
  const rounds = scrollElement.querySelectorAll<HTMLElement>(
    "[data-conversation-round-index]",
  );
  for (const round of rounds) {
    const bounds = round.getBoundingClientRect();
    if (bounds.bottom > viewportTop + VIRTUAL_ANCHOR_TOLERANCE_PX) {
      return {
        index: Number(round.dataset.conversationRoundIndex),
        offsetTop: bounds.top - viewportTop,
      };
    }
  }
  return null;
}

function restoreStaticReadingAnchor<
  TScrollElement extends Element | Window,
  TItemElement extends Element,
>(
  instance: Virtualizer<TScrollElement, TItemElement>,
  scrollElement: Element,
  anchor: ConversationVirtualReadingAnchor,
): void {
  const round = instance.elementsCache.get(
    instance.options.getItemKey(anchor.index),
  );
  if (!round?.isConnected) {
    return;
  }
  const offsetTop = (
    round.getBoundingClientRect().top
    - scrollElement.getBoundingClientRect().top
  );
  if (offsetTop !== anchor.offsetTop) {
    shiftConversationScrollTop(
      scrollElement,
      "virtualizer",
      offsetTop - anchor.offsetTop,
    );
  }
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
