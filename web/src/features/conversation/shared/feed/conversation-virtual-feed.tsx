/**
 * INPUT: DM 轮次数据、渲染器与共享滚动/feed refs。
 * OUTPUT: 使用稳定身份、可拉取的未加载占位、真实轨道估高和可见锚点策略的虚拟消息流。
 * POS: 普通会话超过虚拟化阈值后的 Feed 渲染入口。
 */
import { useCallback, useMemo } from "react";
import { useChatTypography } from "@/shared/theme/chat-typography";
import { useVirtualizer } from "@tanstack/react-virtual";

import { useRoundHeights } from "@/hooks/conversation/use-message-height";

import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "../conversation-panel-styles";
import { getConversationRoundNavigationTarget } from "../timeline/scroll/round-scroll";
import { scrollConversationElementTo } from "../timeline/scroll/scroll-controller";
import {
  isConversationRoundActivelyGrowing,
  resolveConversationRound,
  type ConversationFeedProps,
} from "./conversation-feed-model";
import { ConversationFeedTail } from "./conversation-feed-tail";
import { ConversationVirtualCanvas } from "./conversation-virtual-canvas";
import { ConversationRound } from "./conversation-round";
import { useConversationRoundNavigation } from "./use-conversation-round-navigation";
import { useConversationVirtualMetrics } from "./use-conversation-virtual-metrics";
import {
  scrollConversationVirtualElement,
  shouldAdjustConversationVirtualScrollPosition,
  useConversationVirtualTopologyAnchor,
  useConversationVirtualHandoff,
  useConversationVirtualRemeasure,
  useConversationVirtualItemKey,
} from "./use-conversation-virtual-scroll-policy";

type ConversationVirtualFeedProps = ConversationFeedProps & {
  refs: ConversationFeedProps["refs"] & {
    scrollRef: NonNullable<ConversationFeedProps["refs"]["scrollRef"]>;
  };
};

export function ConversationVirtualFeed({
  isMobileLayout,
  refs,
  renderer,
  source,
}: ConversationVirtualFeedProps) {
  const typography = useChatTypography((state) => state.typography);
  const metrics = useConversationVirtualMetrics(
    refs.scrollRef,
    refs.feedRef,
  );
  const roundNodeIds = useMemo(
    () => source.roundIds.map(
      (_roundId, index) => resolveConversationRound(source, index).nodeId,
    ),
    [source],
  );
  const getItemKey = useConversationVirtualItemKey(roundNodeIds);
  const activeRoundIds = useMemo(() => new Set(
    source.roundIds.filter((_roundId, index) => isConversationRoundActivelyGrowing(
      source,
      resolveConversationRound(source, index),
    )),
  ), [source]);
  const heightMap = useRoundHeights(
    source.roundIds,
    source.messageGroups,
    metrics.containerWidth,
    activeRoundIds,
  );
  const estimateSize = (index: number) => heightMap.get(source.roundIds[index]) ?? 200;
  const shouldHoldReading = () => !(
    (refs.isFollowingLatest?.() ?? false)
    || (refs.isBottomScrollActive?.() ?? false)
    || Boolean(
      refs.scrollRef.current
      && getConversationRoundNavigationTarget(refs.scrollRef.current),
    )
  );
  const handoff = useConversationVirtualHandoff(
    refs.scrollRef,
    estimateSize,
    shouldHoldReading,
  );
  const virtualizer = useVirtualizer({
    count: source.roundIds.length,
    estimateSize,
    getItemKey,
    getScrollElement: () => refs.scrollRef.current,
    scrollToFn: scrollConversationVirtualElement,
    initialOffset: handoff.initialOffset,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 5,
    scrollPaddingStart: metrics.scrollPaddingStart,
  });
  const getTopologyAnchorIndex = useConversationVirtualTopologyAnchor(
    virtualizer,
    roundNodeIds,
    shouldHoldReading,
    handoff.readingAnchor,
  );
  useConversationVirtualRemeasure(virtualizer, typography);
  virtualizer.shouldAdjustScrollPositionOnItemSizeChange = (
    item,
    delta,
    instance,
  ) => shouldAdjustConversationVirtualScrollPosition(
    item,
    delta,
    instance,
    {
      bottomScrollActive: refs.isBottomScrollActive?.() ?? false,
      followingLatest: refs.isFollowingLatest?.() ?? false,
      navigationActive: Boolean(
        refs.scrollRef.current
        && getConversationRoundNavigationTarget(refs.scrollRef.current),
      ),
      topologyAnchorIndex: getTopologyAnchorIndex(),
      userScrollActive: refs.isUserScrollActive?.() ?? false,
    },
  );
  const scrollToIndex = useCallback((
    index: number,
    options?: { behavior?: ScrollBehavior },
  ) => {
    if (index === 0) {
      const scrollElement = refs.scrollRef.current;
      if (scrollElement) {
        scrollConversationElementTo(scrollElement, "navigation", {
          behavior: options?.behavior ?? "smooth",
          top: 0,
        });
      }
      return;
    }
    virtualizer.scrollToIndex(index, {
      align: "start",
      behavior: options?.behavior ?? "smooth",
    });
  }, [refs.scrollRef, virtualizer]);
  useConversationRoundNavigation({
    fallbackScrollToIndex: scrollToIndex,
    roundIds: source.roundIds,
    roundScrollRef: refs.roundScrollRef,
    scrollRef: refs.scrollRef,
  });

  const virtualItems = virtualizer.getVirtualItems();
  const totalSize = virtualizer.getTotalSize();
  return (
    <div
      ref={refs.feedRef}
      data-conversation-virtual-feed="true"
      className={
        isMobileLayout
          ? "nexus-chat-feed relative"
          : `nexus-chat-feed relative ${CONVERSATION_CONTENT_LANE_CLASS_NAME}`
      }
      style={{ height: totalSize }}
    >
      <ConversationVirtualCanvas
        offset={virtualItems[0]?.start ?? 0}
        totalSize={totalSize}
      >
        {virtualItems.map((item) => {
          const state = resolveConversationRound(source, item.index);
          return (
            <ConversationRound
              key={state.nodeId}
              measureRef={virtualizer.measureElement}
              renderer={renderer}
              source={source}
              state={state}
            />
          );
        })}
      </ConversationVirtualCanvas>
      <ConversationFeedTail
        bottomAnchorRef={refs.bottomAnchorRef}
        className="absolute bottom-0 h-px w-full"
      />
    </div>
  );
}
