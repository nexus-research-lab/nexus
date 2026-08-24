/**
 * INPUT: Room 完整会话目录、当前会话与选择命令。
 * OUTPUT: 排除内部草稿但保留外部 Session 的移动端历史切换器。
 * POS: Room 窄窗历史投影视图，不维护标签目录、路由或选中真相。
 */

import { X } from "lucide-react";

import { formatRelativeTime } from "@/lib/format/relative-time";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { RoomConversationView } from "@/types/conversation/conversation";

import { filterRoomHistoryConversations } from "../history/room-history-model";

interface RoomMobileConversationSwitcherProps {
  activeConversationId: string | null;
  conversations: RoomConversationView[];
  isOpen: boolean;
  onClose: () => void;
  onSelect: (conversationId: string) => void;
}

export function RoomMobileConversationSwitcher({
  activeConversationId,
  conversations,
  isOpen,
  onClose,
  onSelect,
}: RoomMobileConversationSwitcherProps) {
  const { t } = useI18n();
  const historyConversations = filterRoomHistoryConversations(conversations);
  if (!isOpen) {
    return null;
  }

  return (
    <>
      <button
        aria-label={t("common.close")}
        className="absolute inset-x-0 bottom-0 top-[52px] z-30 bg-(--dialog-backdrop-color) backdrop-blur-[1px] animate-in fade-in-0 duration-(--motion-duration-fast)"
        onClick={onClose}
        type="button"
      />

      <section
        aria-labelledby="mobile-conversation-switcher-title"
        aria-modal="true"
        className="absolute inset-x-0 top-[52px] z-40 flex max-h-[56dvh] flex-col overflow-hidden rounded-b-[16px] border-b border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,transparent)] bg-[color:color-mix(in_srgb,var(--background)_84%,var(--surface-panel-background)_16%)] shadow-(--surface-popover-shadow) backdrop-blur-[20px] animate-in fade-in-0 slide-in-from-top-2 duration-(--motion-duration-fast)"
        role="dialog"
      >
        <header className="flex min-h-12 shrink-0 items-center justify-between gap-3 border-b divider-subtle px-4 py-2">
          <h2
            className="truncate text-sm font-semibold text-(--text-strong)"
            id="mobile-conversation-switcher-title"
          >
            {t("room.switch_conversation")}
          </h2>

          <button
            aria-label={t("common.close")}
            className="inline-flex h-8 w-8 items-center justify-center rounded-[9px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)"
            onClick={onClose}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </header>

        <div className="soft-scrollbar min-h-0 flex-1 space-y-1 overflow-y-auto p-2.5">
          {historyConversations.map((conversation) => {
            const isActive = conversation.conversation_id === activeConversationId;
            return (
              <button
                key={conversation.conversation_id}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "group relative flex min-h-12 w-full items-center gap-3 overflow-hidden rounded-[10px] px-3 py-2 text-left transition-[background-color,color] duration-(--motion-duration-fast)",
                  isActive
                    ? "bg-(--surface-sidebar-active-background) text-(--text-strong)"
                    : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                )}
                onClick={() => {
                  onSelect(conversation.conversation_id);
                  onClose();
                }}
                type="button"
              >
                <div className="min-w-0 flex-1">
                  <p className={cn(
                    "truncate text-[14px]",
                    isActive
                      ? "font-semibold text-(--text-strong)"
                      : "font-medium text-(--text-default) group-hover:text-(--text-strong)",
                  )}>
                    {conversation.title?.trim() || t("room.new_conversation")}
                  </p>
                  <span className={cn(
                    "mt-0.5 block text-xs",
                    isActive ? "text-(--text-muted)" : "text-(--text-soft)",
                  )}>
                    {formatRelativeTime(conversation.last_activity_at)}
                  </span>
                </div>
              </button>
            );
          })}
        </div>
      </section>
    </>
  );
}
