/**
 * INPUT: Room 轮次投影、渲染器与共享滚动/feed refs。
 * OUTPUT: 使用稳定身份、真实内容高度、pending slot 估高和可见锚点策略的群聊虚拟消息流。
 * POS: Room 会话超过虚拟化阈值后的 Feed 渲染入口。
 */
import { useCallback, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { ConversationFeedTail } from "@/features/conversation/shared/feed/conversation-feed-tail";
import { useConversationRoundNavigation } from "@/features/conversation/shared/feed/use-conversation-round-navigation";
import { useConversationVirtualMetrics } from "@/features/conversation/shared/feed/use-conversation-virtual-metrics";
import {
  shouldAdjustConversationVirtualScrollPosition,
  useConversationVirtualInitialOffset,
  useConversationVirtualItemKey,
} from "@/features/conversation/shared/feed/use-conversation-virtual-scroll-policy";
import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import { estimateRoundHeights } from "@/hooks/conversation/use-message-height";

import {
  buildGroupConversationRoundAliases,
  isGroupConversationRoundActivelyGrowing,
  resolveGroupConversationRound,
  type GroupConversationFeedProps,
} from "./group-conversation-feed-model";
import { projectGroupRoundHeights } from "./group-conversation-height-model";
import { GroupConversationRound } from "./group-conversation-round";

type GroupConversationVirtualFeedProps = GroupConversationFeedProps & {
  refs: GroupConversationFeedProps["refs"] & {
    scrollRef: NonNullable<GroupConversationFeedProps["refs"]["scrollRef"]>;
  };
};

export function GroupConversationVirtualFeed({
  isMobileLayout,
  refs,
  renderer,
  source,
}: GroupConversationVirtualFeedProps) {
  const metrics = useConversationVirtualMetrics(
    refs.scrollRef,
    refs.feedRef,
  );
  const getItemKey = useConversationVirtualItemKey(source.roundIds);
  const initialOffset = useConversationVirtualInitialOffset(refs.scrollRef);
  const roundIdAliases = useMemo(
    () => buildGroupConversationRoundAliases(source),
    [source],
  );
  const activeRoundIds = useMemo(() => new Set(
    source.roundIds.filter((_roundId, index) => isGroupConversationRoundActivelyGrowing(
      source,
      resolveGroupConversationRound(source, index),
    )),
  ), [source]);

  const heightMap = useMemo(() => {
    const baseHeights = estimateRoundHeights(
      source.roundIds,
      source.messageGroups,
      metrics.containerWidth,
      activeRoundIds,
    );
    return projectGroupRoundHeights({
      baseHeights,
      messageGroups: source.messageGroups,
      pendingSlotGroups: source.pendingSlotGroups,
      roundIds: source.roundIds,
    });
  }, [
    activeRoundIds,
    metrics.containerWidth,
    source.messageGroups,
    source.pendingSlotGroups,
    source.roundIds,
  ]);
  const virtualizer = useVirtualizer({
    count: source.roundIds.length,
    estimateSize: (index) => heightMap.get(source.roundIds[index]) ?? 200,
    getItemKey,
    getScrollElement: () => refs.scrollRef.current,
    initialOffset,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 5,
    scrollPaddingStart: metrics.scrollPaddingStart,
  });
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
    },
  );
  const scrollToIndex = useCallback(
    (index: number, options?: { behavior?: ScrollBehavior }) => {
      if (index === 0) {
        refs.scrollRef.current?.scrollTo({
          behavior: options?.behavior ?? "smooth",
          top: 0,
        });
        return;
      }
      virtualizer.scrollToIndex(index, {
        align: "start",
        behavior: options?.behavior ?? "smooth",
      });
    },
    [refs.scrollRef, virtualizer],
  );
  useConversationRoundNavigation({
    fallbackScrollToIndex: scrollToIndex,
    roundIds: source.roundIds,
    roundIdAliases,
    roundScrollRef: refs.roundScrollRef,
    scrollRef: refs.scrollRef,
  });

  const virtualItems = virtualizer.getVirtualItems();
  return (
    <div
      ref={refs.feedRef}
      data-conversation-virtual-feed="true"
      className={
        isMobileLayout
          ? "nexus-chat-feed relative"
          : `nexus-chat-feed relative ${CONVERSATION_CONTENT_LANE_CLASS_NAME}`
      }
      style={{ height: virtualizer.getTotalSize() }}
    >
      <div
        className="absolute left-0 top-0 w-full"
        style={{ transform: `translateY(${virtualItems[0]?.start ?? 0}px)` }}
      >
        {virtualItems.map((item) => {
          const state = resolveGroupConversationRound(source, item.index);
          return (
            <GroupConversationRound
              isMobileLayout={isMobileLayout}
              key={state.roundId}
              measureRef={virtualizer.measureElement}
              renderer={renderer}
              state={state}
            />
          );
        })}
      </div>
      <ConversationFeedTail
        bottomAnchorRef={refs.bottomAnchorRef}
        className="absolute bottom-0 h-px w-full"
      />
    </div>
  );
}
