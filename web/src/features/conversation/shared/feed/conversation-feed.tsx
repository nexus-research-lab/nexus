/**
 * INPUT: DM 轮次 source、可选同栈前导内容、渲染器与共享滚动 refs。
 * OUTPUT: 含本地前导内容的静态 Feed或普通虚拟 Feed，并以真实内容高度驱动贴底增长。
 * POS: DM 主消息流的分支装配入口。
 */
import { memo, useRef } from "react";

import { CONVERSATION_CONTENT_LANE_CLASS_NAME } from "../conversation-panel-styles";
import {
  resolveConversationRound,
  type ConversationFeedProps,
} from "./conversation-feed-model";
import { ConversationFeedTail } from "./conversation-feed-tail";
import { ConversationRound } from "./conversation-round";
import { ConversationVirtualFeed } from "./conversation-virtual-feed";
import { useConversationRoundNavigation } from "./use-conversation-round-navigation";
import { useConversationVirtualizationPolicy } from "./use-conversation-virtualization-policy";

const VIRTUAL_ROUND_THRESHOLD = 20;

export const ConversationFeed = memo(function ConversationFeed(
  props: ConversationFeedProps,
) {
  const virtualizationEnabled = useConversationVirtualizationPolicy({
    active: props.source.liveLayoutActive,
    count: props.source.roundIds.length,
    scopeKey: props.source.scopeKey,
    threshold: VIRTUAL_ROUND_THRESHOLD,
  });
  const shouldVirtualize = props.leadingContent == null
    && virtualizationEnabled
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
  leadingContent,
  refs,
  renderer,
  source,
}: ConversationFeedProps) {
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
          ? "nexus-chat-feed flex flex-col"
          : `nexus-chat-feed ${CONVERSATION_CONTENT_LANE_CLASS_NAME} flex flex-col`
      }
    >
      {leadingContent}
      {source.roundIds.map((_roundId, index) => {
        const state = resolveConversationRound(source, index);
        return (
          <ConversationRound
            isMobileLayout={isMobileLayout}
            key={state.nodeId}
            renderer={renderer}
            source={source}
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
