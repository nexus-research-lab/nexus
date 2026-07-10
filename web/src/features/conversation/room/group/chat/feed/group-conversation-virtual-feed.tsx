import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

import { getConversationRoundFocusOffset } from "@/features/conversation/shared/conversation-round-scroll";
import { estimateRoundHeights } from "@/hooks/conversation/use-message-height";

import {
  buildRoundIndexItemMap,
  resolveGroupConversationRound,
  type GroupConversationFeedProps,
} from "./group-conversation-feed-model";
import { GroupConversationRound } from "./group-conversation-round";
import { useGroupConversationRoundNavigation } from "./use-group-conversation-round-navigation";

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
  const containerWidthRef = useRef(680);
  const [scrollPaddingStart, setScrollPaddingStart] = useState(180);
  const roundIndexItemById = useMemo(
    () => buildRoundIndexItemMap(source.roundIndexItems),
    [source.roundIndexItems],
  );

  useEffect(() => {
    const scrollElement = refs.scrollRef.current;
    if (!scrollElement) {
      return;
    }
    const syncScrollMetrics = () => {
      containerWidthRef.current = scrollElement.clientWidth || 680;
      const nextPaddingStart = getConversationRoundFocusOffset(scrollElement);
      setScrollPaddingStart((current) =>
        current === nextPaddingStart ? current : nextPaddingStart,
      );
    };
    syncScrollMetrics();
    const observer = new ResizeObserver(syncScrollMetrics);
    observer.observe(scrollElement);
    return () => observer.disconnect();
  }, [refs.scrollRef]);

  const heightMap = useMemo(
    () =>
      estimateRoundHeights(
        source.roundIds,
        source.messageGroups,
        containerWidthRef.current,
      ),
    [source.messageGroups, source.roundIds],
  );
  const virtualizer = useVirtualizer({
    count: source.roundIds.length,
    estimateSize: (index) => heightMap.get(source.roundIds[index]) ?? 200,
    getScrollElement: () => refs.scrollRef.current,
    measureElement: (element) => element.getBoundingClientRect().height,
    overscan: 5,
    scrollPaddingStart,
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
  useGroupConversationRoundNavigation({
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
          const state = resolveGroupConversationRound(source, item.index);
          return (
            <GroupConversationRound
              key={state.roundId}
              indexItem={roundIndexItemById.get(state.roundId)}
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
