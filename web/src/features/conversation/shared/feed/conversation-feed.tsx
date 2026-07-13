import { memo, useMemo, useRef } from "react";

import {
  buildRoundIndexItemMap,
  resolveConversationRound,
  type ConversationFeedProps,
} from "./conversation-feed-model";
import { ConversationRound } from "./conversation-round";
import { ConversationVirtualFeed } from "./conversation-virtual-feed";
import { useConversationRoundNavigation } from "./use-conversation-round-navigation";

const VIRTUAL_ROUND_THRESHOLD = 20;

export const ConversationFeed = memo(function ConversationFeed(
  props: ConversationFeedProps,
) {
  const shouldVirtualize =
    props.source.roundIds.length >= VIRTUAL_ROUND_THRESHOLD
    && Boolean(props.refs.scrollRef);

  if (shouldVirtualize && props.refs.scrollRef) {
    return (
      <ConversationVirtualFeed
        {...props}
        refs={{ ...props.refs, scrollRef: props.refs.scrollRef }}
      />
    );
  }
  return <StaticConversationFeed {...props} />;
});

function StaticConversationFeed({
  isMobileLayout,
  refs,
  renderer,
  source,
}: ConversationFeedProps) {
  const roundIndexItemById = useMemo(
    () => buildRoundIndexItemMap(source.roundIndexItems),
    [source.roundIndexItems],
  );
  const unavailableScrollRef = useRef<HTMLDivElement>(null);
  useConversationRoundNavigation({
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
        const state = resolveConversationRound(source, index);
        return (
          <ConversationRound
            key={roundId}
            indexItem={roundIndexItemById.get(roundId)}
            renderer={renderer}
            source={source}
            state={state}
          />
        );
      })}
      <div ref={refs.bottomAnchorRef} className="h-px w-full" />
    </div>
  );
}
