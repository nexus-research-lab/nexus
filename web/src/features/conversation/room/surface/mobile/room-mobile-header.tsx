import { ArrowLeft, Bot, ChevronDown } from "lucide-react";

import { getIconAvatarSrc, getInitials } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";

interface RoomMobileHeaderProps {
  agentAvatar?: string | null;
  agentName: string;
  canOpenSubagents: boolean;
  conversationTitle: string;
  onBack: () => void;
  onOpenConversations: () => void;
  onOpenSubagents: () => void;
  roomTitle: string;
}

export function RoomMobileHeader({
  agentAvatar,
  agentName,
  canOpenSubagents,
  conversationTitle,
  onBack,
  onOpenConversations,
  onOpenSubagents,
  roomTitle,
}: RoomMobileHeaderProps) {
  const { t } = useI18n();
  const avatarSrc = getIconAvatarSrc(agentAvatar);

  return (
    <div className="px-2 pb-2 pt-2">
      <div className="surface-radius-lg flex items-center gap-2 px-2 py-2">
        <button
          aria-label={t("common.back")}
          className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl text-(--text-strong) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
          onClick={onBack}
          type="button"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>

        <button
          aria-label={t("room.switch_conversation")}
          className="flex min-w-0 flex-1 items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) px-3 py-2 text-left transition hover:bg-(--interaction-hover-background)"
          onClick={onOpenConversations}
          type="button"
        >
          <div className="flex h-8 w-8 shrink-0 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[11px] font-bold text-(--text-strong) shadow-(--surface-avatar-shadow)">
            {avatarSrc ? (
              <img
                alt={agentName}
                className="h-full w-full object-cover"
                src={avatarSrc}
              />
            ) : (
              getInitials(agentName, "AI", 2)
            )}
          </div>

          <div className="min-w-0 flex-1">
            <p className="truncate text-sm font-semibold text-(--text-strong)">
              {agentName}
            </p>
            <p className="truncate text-[12px] text-(--text-muted)">
              {roomTitle || conversationTitle}
            </p>
          </div>

          <ChevronDown className="h-4 w-4 shrink-0 text-(--text-muted)" />
        </button>

        <button
          aria-label={t("subagents.open_panel")}
          className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-2xl border border-(--divider-subtle-color) text-(--text-muted) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)"
          disabled={!canOpenSubagents}
          onClick={onOpenSubagents}
          title={t("subagents.open_panel")}
          type="button"
        >
          <Bot className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
