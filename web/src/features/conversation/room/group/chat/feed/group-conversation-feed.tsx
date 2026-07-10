import { memo, useMemo, useRef } from "react";

import {
  buildRoundIndexItemMap,
  resolveGroupConversationRound,
  type GroupConversationFeedProps,
} from "./group-conversation-feed-model";
import { GroupConversationRound } from "./group-conversation-round";
import { GroupConversationVirtualFeed } from "./group-conversation-virtual-feed";
import { useGroupConversationRoundNavigation } from "./use-group-conversation-round-navigation";

const VIRTUAL_ROUND_THRESHOLD = 20;

export const GroupConversationFeed = memo(function GroupConversationFeed(
  props: GroupConversationFeedProps,
) {
  const { isMobileLayout, refs, renderer, source } = props;
  const shouldVirtualize =
    source.roundIds.length >= VIRTUAL_ROUND_THRESHOLD && Boolean(refs.scrollRef);

  if (shouldVirtualize && refs.scrollRef) {
    return (
      <GroupConversationVirtualFeed
        {...props}
        refs={{ ...refs, scrollRef: refs.scrollRef }}
      />
    );
  }

  return (
    <StaticGroupConversationFeed
      isMobileLayout={isMobileLayout}
      refs={refs}
      renderer={renderer}
      source={source}
    />
  );
});

function StaticGroupConversationFeed({
  isMobileLayout,
  refs,
  renderer,
  source,
}: GroupConversationFeedProps) {
  const roundIndexItemById = useMemo(
    () => buildRoundIndexItemMap(source.roundIndexItems),
    [source.roundIndexItems],
  );
  const unavailableScrollRef = useRef<HTMLDivElement>(null);
  useGroupConversationRoundNavigation({
    roundIds: source.roundIds,
    roundScrollRef: refs.roundScrollRef,
    scrollRef: refs.scrollRef ?? unavailableScrollRef,
  });

  return (
    <div
      ref={refs.feedRef}
      className={
        isMobileLayout
          ? "nexus-chat-feed space-y-4"
          : "nexus-chat-feed mx-auto flex w-full max-w-[980px] flex-col gap-1"
      }
    >
      {source.roundIds.map((roundId, index) => {
        const state = resolveGroupConversationRound(source, index);
        return (
          <GroupConversationRound
            key={roundId}
            indexItem={roundIndexItemById.get(roundId)}
            renderer={renderer}
            state={state}
          />
        );
      })}
      <div ref={refs.bottomAnchorRef} className="h-px w-full" />
    </div>
  );
}
