import { Check, Clock3, MessageSquare, X } from "lucide-react";

import { formatRelativeTime } from "@/lib/format/relative-time";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type { RoomConversationView } from "@/types/conversation/conversation";

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
        className="absolute inset-x-0 top-[52px] z-40 flex max-h-[56dvh] flex-col overflow-hidden rounded-b-[16px] border-b border-(--surface-panel-border) bg-(--surface-panel-background) shadow-[0_16px_34px_rgba(15,23,42,0.14)] animate-in fade-in-0 slide-in-from-top-2 duration-(--motion-duration-fast)"
        role="dialog"
      >
        <header className="flex min-h-14 shrink-0 items-center justify-between gap-3 border-b divider-subtle px-4 py-2.5">
          <div className="min-w-0">
            <h2
              className="truncate text-[13px] font-semibold text-(--text-strong)"
              id="mobile-conversation-switcher-title"
            >
              {t("room.switch_conversation")}
            </h2>
            <p className="mt-0.5 text-[10.5px] text-(--text-soft)">
              {t("room.conversation_count", { count: conversations.length })}
            </p>
          </div>

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
          {conversations.map((conversation) => {
            const isActive = conversation.conversation_id === activeConversationId;
            return (
              <button
                key={conversation.conversation_id}
                aria-current={isActive ? "page" : undefined}
                className={cn(
                  "group relative flex min-h-[68px] w-full items-center gap-3 overflow-hidden rounded-[12px] border border-transparent px-3 py-2.5 text-left transition-[background-color,border-color] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] hover:bg-(--surface-interactive-hover-background)",
                  isActive && "border-[color:color-mix(in_srgb,var(--primary)_22%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-interactive-active-background)_56%,transparent)]",
                )}
                onClick={() => {
                  onSelect(conversation.conversation_id);
                  onClose();
                }}
                type="button"
              >
                <span
                  aria-hidden="true"
                  className={cn(
                    "absolute bottom-2 left-0 top-2 w-0.5 rounded-full",
                    isActive ? "bg-(--primary)" : "bg-transparent",
                  )}
                />

                <span className={cn(
                  "flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_50%,transparent)] text-(--icon-muted)",
                  isActive && "border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] text-(--primary)",
                )}>
                  <MessageSquare className="h-[18px] w-[18px]" />
                </span>

                <div className="min-w-0 flex-1">
                  <p className="truncate text-[14px] font-semibold text-(--text-strong)">
                    {conversation.title?.trim() || t("room.untitled_conversation")}
                  </p>
                  <span className="mt-1 flex items-center gap-1.5 text-[11px] text-(--text-soft)">
                    <Clock3 className="h-3 w-3 shrink-0" />
                    {formatRelativeTime(conversation.last_activity_at)}
                  </span>
                </div>

                {isActive ? (
                  <span className="inline-flex shrink-0 items-center gap-1 rounded-[7px] border border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] px-1.5 py-1 text-[9.5px] font-medium text-(--primary)">
                    <Check className="h-3 w-3" />
                    {t("room.current_conversation")}
                  </span>
                ) : null}
              </button>
            );
          })}
        </div>
      </section>
    </>
  );
}
