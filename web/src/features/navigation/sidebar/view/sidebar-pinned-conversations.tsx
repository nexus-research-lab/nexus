/**
 * INPUT: 上层投影的固定会话、活动态与导航/取消固定动作。
 * OUTPUT: 能力入口下方带分割线、独立取消位与滚动边界的固定会话 Dock。
 * POS: 主侧栏导航轨的固定会话纯视图，不读取 Store、路由或业务 API。
 */
import { MessageSquareText, X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { SIDEBAR_SELECTION_CLASS_NAME } from "@/shared/ui/sidebar/sidebar-selection";

import type { SidebarPinnedConversationItem } from "./sidebar-wide-panel-types";

interface SidebarPinnedConversationsProps {
  items: SidebarPinnedConversationItem[];
  label: string;
  onSelect: (item: SidebarPinnedConversationItem) => void;
  onUnpin: (item: SidebarPinnedConversationItem) => void;
  unpinLabel: string;
}

export function SidebarPinnedConversations({
  items,
  label,
  onSelect,
  onUnpin,
  unpinLabel,
}: SidebarPinnedConversationsProps) {
  if (items.length === 0) {
    return null;
  }

  return (
    <section
      aria-label={label}
      className="soft-scrollbar min-h-0 w-full flex-1 overflow-y-auto overscroll-contain px-1 pb-2"
      data-pinned-conversation-scroll-region="true"
    >
      <div className="mx-auto flex w-14 flex-col items-center gap-2 pt-3">
        <div
          aria-hidden="true"
          className="h-px w-10 shrink-0 bg-(--divider-subtle-color)"
        />
        {items.map((item) => (
          <div
            className={cn(
              "group/pinned relative h-[58px] w-14 shrink-0 rounded-[12px] transition-[background,color] duration-(--motion-duration-fast)",
              item.active
                ? SIDEBAR_SELECTION_CLASS_NAME
                : "hover:bg-(--surface-interactive-hover-background)",
            )}
            data-pinned-conversation-id={item.conversationId}
            key={item.key}
          >
            <button
              aria-current={item.active ? "page" : undefined}
              className="absolute inset-0 min-w-0 rounded-[12px] text-2xs font-medium text-(--text-muted) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_34%,transparent)]"
              onClick={() => onSelect(item)}
              title={item.title}
              type="button"
            >
              <span className="absolute left-0 top-0 flex h-8 w-8 items-center justify-center">
                <MessageSquareText className="h-[17px] w-[17px]" />
              </span>
              <span className="absolute inset-x-0 bottom-1 block truncate px-1 text-center leading-tight">
                {item.title}
              </span>
            </button>
            <button
              aria-label={`${unpinLabel}：${item.title}`}
              className="absolute right-0 top-0 z-10 flex h-6 w-6 items-center justify-center rounded-full text-(--icon-muted) opacity-0 transition-[background-color,color,opacity] duration-(--motion-duration-fast) hover:bg-(--surface-interactive-active-background) hover:text-(--text-strong) focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_34%,transparent)] group-focus-within/pinned:opacity-100 group-hover/pinned:opacity-100"
              data-pinned-conversation-unpin="true"
              onClick={() => onUnpin(item)}
              title={unpinLabel}
              type="button"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        ))}
      </div>
    </section>
  );
}
