import { useCallback, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { useConversationRoundNavigation } from "@/features/conversation/shared/feed/use-conversation-round-navigation";
import { useConversationVirtualMetrics } from "@/features/conversation/shared/feed/use-conversation-virtual-metrics";
import { estimateRoundHeights } from "@/hooks/conversation/use-message-height";

import {
  buildGroupConversationRoundAliases,
  resolveGroupConversationRound,
  type GroupConversationFeedProps,
} from "./group-conversation-feed-model";
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
  const metrics = useConversationVirtualMetrics(refs.scrollRef);
  const roundIdAliases = useMemo(
    () => buildGroupConversationRoundAliases(source),
    [source],
  );

  const heightMap = useMemo(
    () =>
      estimateRoundHeights(
        source.roundIds,
        source.messageGroups,
        metrics.containerWidth,
      ),
    [metrics.containerWidth, source.messageGroups, source.roundIds],
  );
  const virtualizer = useVirtualizer({
    count: source.roundIds.length,
    estimateSize: (index) => heightMap.get(source.roundIds[index]) ?? 200,
    getItemKey: (index) => source.roundIds[index],
    getScrollElement: () => refs.scrollRef.current,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 5,
    scrollPaddingStart: metrics.scrollPaddingStart,
  });
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
      className={
        isMobileLayout
          ? "nexus-chat-feed relative"
          : "nexus-chat-feed relative mx-auto w-full max-w-[980px]"
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
              key={state.roundId}
              measureRef={virtualizer.measureElement}
              renderer={renderer}
              state={state}
            />
          );
        })}
      </div>
      <div
        ref={refs.bottomAnchorRef}
        className="absolute bottom-0 h-px w-full"
      />
    </div>
  );
}
