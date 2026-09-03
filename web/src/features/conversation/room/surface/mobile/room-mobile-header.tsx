/**
 * INPUT: 窄窗 Room 身份、会话切换状态、返回动作与尾部操作。
 * OUTPUT: 带拖窗热区和非交互下缘渐隐的专注模式 Header。
 * POS: Room 专注模式顶部导航；只承载导航，不改变聊天 viewport 几何。
 */
import { ArrowLeft, ChevronDown } from "lucide-react";
import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";

import "../room-conversation-header-edge.css";

interface RoomMobileHeaderProps {
  conversationTitle: string;
  isConversationSwitcherOpen: boolean;
  onBack: () => void;
  onOpenConversations: () => void;
  roomTitle: string;
  trailing: ReactNode;
}

export function RoomMobileHeader({
  conversationTitle,
  isConversationSwitcherOpen,
  onBack,
  onOpenConversations,
  roomTitle,
  trailing,
}: RoomMobileHeaderProps) {
  const { t } = useI18n();
  const primaryTitle = roomTitle.trim() || conversationTitle;
  const secondaryTitle = conversationTitle !== primaryTitle
    ? conversationTitle
    : null;

  return (
    <header
      className="nexus-room-conversation-header-edge nexus-room-conversation-header-edge--mobile flex h-[52px] shrink-0 items-center gap-1.5 border-b divider-subtle px-2 sm:px-3"
      data-desktop-window-controls-leading
      data-desktop-window-drag-region
      data-room-conversation-header-edge="true"
    >
      <button
        aria-label={t("common.back")}
        className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-(--text-strong) transition hover:bg-(--interaction-hover-background)"
        onClick={onBack}
        type="button"
      >
        <ArrowLeft className="h-4 w-4" />
      </button>

      <button
        aria-expanded={isConversationSwitcherOpen}
        aria-haspopup="dialog"
        aria-label={t("room.switch_conversation")}
        className={cn(
          "flex min-w-0 flex-1 items-center gap-1.5 rounded-[10px] px-2 py-1 text-left transition-colors hover:bg-(--interaction-hover-background)",
          isConversationSwitcherOpen && "bg-(--surface-interactive-hover-background)",
        )}
        onClick={onOpenConversations}
        title={conversationTitle}
        type="button"
      >
        <div className="min-w-0 flex-1">
          <p className="truncate text-base font-semibold leading-5 text-(--text-strong)">
            {primaryTitle}
          </p>
          {secondaryTitle ? (
            <p className="truncate text-xs leading-4 text-(--text-soft)">
              {secondaryTitle}
            </p>
          ) : null}
        </div>
        <ChevronDown className={cn(
          "h-3.5 w-3.5 shrink-0 text-(--text-muted) transition-transform duration-(--motion-duration-fast)",
          isConversationSwitcherOpen && "rotate-180",
        )} />
      </button>

      <div className="flex shrink-0 items-center gap-0.5">
        {trailing}
      </div>
    </header>
  );
}
