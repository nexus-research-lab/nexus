import { Clock3, MessageSquarePlus } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

interface RoomHistoryEmptyStateProps {
  canCreateConversation: boolean;
  onCreateConversation: () => void;
}

export function RoomHistoryEmptyState({
  canCreateConversation,
  onCreateConversation,
}: RoomHistoryEmptyStateProps) {
  const {t} = useI18n();
  return (
    <div className="rounded-[12px] border border-(--divider-subtle-color) px-6 py-10 text-center">
      <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-(--icon-default) shadow-(--surface-avatar-shadow)">
        <Clock3 className="h-4 w-4" />
      </div>
      <p className="mt-4 text-[15px] font-semibold text-(--text-strong)">
        {t("room.no_conversations")}
      </p>
      <p className="mt-1 text-[12px] leading-6 text-(--text-soft)">
        {t("room.history_empty_hint")}
      </p>
      {canCreateConversation ? (
        <button
          className="mt-4 inline-flex items-center gap-1.5 text-[12px] font-semibold text-(--primary) transition duration-(--motion-duration-fast) ease-out hover:text-[color:color-mix(in_srgb,var(--primary)_84%,var(--foreground)_16%)]"
          onClick={onCreateConversation}
          type="button"
        >
          <MessageSquarePlus className="h-3.5 w-3.5" />
          {t("room.new_conversation")}
        </button>
      ) : null}
    </div>
  );
}
