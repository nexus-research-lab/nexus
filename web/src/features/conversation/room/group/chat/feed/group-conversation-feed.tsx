/**
 * INPUT: Room root/Agent 节点 source、渲染器与共享滚动 refs。
 * OUTPUT: 静态/虚拟 Room Feed，并以真实内容高度驱动整组贴底增长。
 * POS: Room 主消息流的分支装配入口。
 */
import { memo, useMemo, useRef } from "react";

import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "@/features/conversation/shared/conversation-panel-styles";
import { ConversationFeedTail } from "@/features/conversation/shared/feed/conversation-feed-tail";
import {
  buildGroupConversationRoundAliases,
  resolveGroupConversationRound,
  type GroupConversationFeedProps,
} from "./group-conversation-feed-model";
import { GroupConversationRound } from "./group-conversation-round";
import { GroupConversationVirtualFeed } from "./group-conversation-virtual-feed";
import { useConversationRoundNavigation } from "@/features/conversation/shared/feed/use-conversation-round-navigation";
import { useConversationVirtualizationPolicy } from "@/features/conversation/shared/feed/use-conversation-virtualization-policy";

const VIRTUAL_ROUND_THRESHOLD = 20;

export const GroupConversationFeed = memo(function GroupConversationFeed(
  props: GroupConversationFeedProps,
) {
  const { isMobileLayout, refs, renderer, source } = props;
  const shouldVirtualize = useConversationVirtualizationPolicy({
    active: source.liveLayoutActive,
    count: source.roundIds.length,
    scopeKey: source.scopeKey,
    threshold: VIRTUAL_ROUND_THRESHOLD,
  }) && Boolean(refs.scrollRef);

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
  const roundIdAliases = useMemo(
    () => buildGroupConversationRoundAliases(source),
    [source],
  );
  const unavailableScrollRef = useRef<HTMLDivElement>(null);
  useConversationRoundNavigation({
    roundIds: source.roundIds,
    roundIdAliases,
    roundScrollRef: refs.roundScrollRef,
    scrollRef: refs.scrollRef ?? unavailableScrollRef,
  });

  return (
    <div
      ref={refs.feedRef}
      className={
        isMobileLayout
          ? "nexus-chat-feed flex flex-col"
          : `nexus-chat-feed ${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col`
      }
    >
      {source.roundIds.map((roundId, index) => {
        const state = resolveGroupConversationRound(source, index);
        return (
          <GroupConversationRound
            isMobileLayout={isMobileLayout}
            key={roundId}
            renderer={renderer}
            state={state}
          />
        );
      })}
      <ConversationFeedTail
        bottomAnchorRef={refs.bottomAnchorRef}
      />
    </div>
  );
}
