/**
 * INPUT: Virtualizer 总高度、首个已挂载 item 偏移与当前可见轮次节点。
 * OUTPUT: 从 Feed 顶部开始的完整虚拟画布，以及在画布内按真实 offset 放置的可见窗口。
 * POS: DM/Room 虚拟 Feed 共用的真实高度定位层，不保留历史测高空白。
 */
import type { ReactNode } from "react";

export function ConversationVirtualCanvas({
  children,
  offset,
  totalSize,
}: {
  children: ReactNode;
  offset: number;
  totalSize: number;
}) {
  return (
    <div
      className="absolute inset-x-0 top-0 w-full"
      data-conversation-virtual-canvas="true"
      style={{ height: totalSize }}
    >
      <div
        className="absolute left-0 top-0 w-full"
        style={{ transform: `translateY(${offset}px)` }}
      >
        {children}
      </div>
    </div>
  );
}
