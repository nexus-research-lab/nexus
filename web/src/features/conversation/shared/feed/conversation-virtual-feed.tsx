import { useCallback, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { estimateRoundHeights } from "@/hooks/conversation/use-message-height";

import {
  buildRoundIndexItemMap,
  resolveConversationRound,
  type ConversationFeedProps,
} from "./conversation-feed-model";
import { ConversationRound } from "./conversation-round";
import { useConversationRoundNavigation } from "./use-conversation-round-navigation";
import { useConversationVirtualMetrics } from "./use-conversation-virtual-metrics";

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
  const metrics = useConversationVirtualMetrics(refs.scrollRef);
  const roundIndexItemById = useMemo(
    () => buildRoundIndexItemMap(source.roundIndexItems),
    [source.roundIndexItems],
  );
  const heightMap = useMemo(
    () => estimateRoundHeights(
      source.roundIds,
      source.messageGroups,
      metrics.containerWidth,
    ),
    [metrics.containerWidth, source.messageGroups, source.roundIds],
  );
  const virtualizer = useVirtualizer({
    count: source.roundIds.length,
    estimateSize: (index) => heightMap.get(source.roundIds[index]) ?? 200,
    getScrollElement: () => refs.scrollRef.current,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 5,
    scrollPaddingStart: metrics.scrollPaddingStart,
  });
  const scrollToIndex = useCallback((
    index: number,
    options?: { behavior?: ScrollBehavior },
  ) => {
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
  }, [refs.scrollRef, virtualizer]);
  useConversationRoundNavigation({
    fallbackScrollToIndex: scrollToIndex,
    roundIds: source.roundIds,
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
          const state = resolveConversationRound(source, item.index);
          return (
            <ConversationRound
              key={state.roundId}
              indexItem={roundIndexItemById.get(state.roundId)}
              measureRef={virtualizer.measureElement}
              renderer={renderer}
              source={source}
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
