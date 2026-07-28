/**
 * INPUT: 共享滚动控制器提供的底部锚点 ref 与可选定位样式。
 * OUTPUT: 不改变内容增长节奏的单一轻量 Feed 尾部锚点。
 * POS: DM、Room 静态与虚拟 Feed 共用的真实内容终点。
 */
import type { RefObject } from "react";

interface ConversationFeedTailProps {
  bottomAnchorRef: RefObject<HTMLDivElement | null>;
  className?: string;
}

export function ConversationFeedTail({
  bottomAnchorRef,
  className = "h-px w-full",
}: ConversationFeedTailProps) {
  return (
    <div
      ref={bottomAnchorRef}
      aria-hidden="true"
      className={className}
      data-conversation-feed-tail
    />
  );
}
