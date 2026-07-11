import { Check, MessageSquare, X } from "lucide-react";

import { formatRelativeTime } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { RoomConversationView } from "@/types/conversation/conversation";

interface RoomMobileConversationSheetProps {
  activeConversationId: string | null;
  conversations: RoomConversationView[];
  isOpen: boolean;
  onClose: () => void;
  onSelect: (conversationId: string) => void;
}

export function RoomMobileConversationSheet({
  activeConversationId,
  conversations,
  isOpen,
  onClose,
  onSelect,
}: RoomMobileConversationSheetProps) {
  const { t } = useI18n();
  if (!isOpen) {
    return null;
  }

  return (
    <>
      <button
        aria-label={t("common.close")}
        className="absolute inset-0 z-30 bg-(--dialog-backdrop-color)"
        onClick={onClose}
        type="button"
      />

      <div
        aria-labelledby="mobile-conversation-sheet-title"
        aria-modal="true"
        className="absolute inset-x-0 bottom-0 z-40 rounded-t-[28px] border-t border-(--surface-panel-border) bg-(--surface-panel-background) px-4 pb-6 pt-3 shadow-[0_-20px_40px_rgba(0,0,0,0.12)]"
        role="dialog"
      >
        <div className="mx-auto mb-4 h-1.5 w-12 rounded-full bg-(--divider-strong-color)" />

        <div className="mb-4 flex items-center justify-between gap-3">
          <div>
            <p
              className="text-sm font-semibold text-(--text-strong)"
              id="mobile-conversation-sheet-title"
            >
              {t("room.switch_conversation")}
            </p>
            <p className="text-xs text-(--text-muted)">
              {t("room.conversation_count", { count: conversations.length })}
            </p>
          </div>

          <button
            aria-label={t("common.close")}
            className="inline-flex h-9 w-9 items-center justify-center rounded-full text-(--text-muted) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
            onClick={onClose}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="max-h-[50vh] space-y-2 overflow-y-auto pr-1">
          {conversations.map((conversation) => {
            const isActive = conversation.conversation_id === activeConversationId;
            return (
              <button
                key={conversation.conversation_id}
                className="flex w-full items-start gap-3 rounded-2xl border border-(--divider-subtle-color) px-3 py-3 text-left transition hover:bg-(--interaction-hover-background)"
                onClick={() => {
                  onSelect(conversation.conversation_id);
                  onClose();
                }}
                type="button"
              >
                <div className="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-(--divider-subtle-color) text-(--text-strong)">
                  {isActive ? (
                    <Check className="h-4 w-4" />
                  ) : (
                    <MessageSquare className="h-4 w-4" />
                  )}
                </div>

                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium text-(--text-strong)">
                    {conversation.title?.trim() || t("room.untitled_conversation")}
                  </p>
                  <p className="mt-1 text-xs text-(--text-muted)">
                    {formatRelativeTime(conversation.last_activity_at)}
                  </p>
                </div>
              </button>
            );
          })}
        </div>
      </div>
    </>
  );
}
