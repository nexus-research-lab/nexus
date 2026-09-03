/**
 * INPUT: 窄窗 Room 身份、会话切换状态、返回动作、尾部操作与共享平台几何。
 * OUTPUT: 与应用二级页头同高、带拖窗热区和非交互下缘渐隐的专注模式 Header。
 * POS: Room 专注模式顶部导航；只承载导航，不改变聊天 viewport 几何。
 */
import { ArrowLeft, ChevronDown } from "lucide-react";
import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
  MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
} from "@/shared/ui/layout/mobile-shell-header-layout";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
      className={cn(
        "nexus-room-conversation-header-edge nexus-room-conversation-header-edge--mobile flex shrink-0 items-center gap-1.5 border-b divider-subtle",
        MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
        MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
      )}
      data-desktop-window-controls-leading
      data-desktop-window-drag-region
      data-room-conversation-header-edge="true"
    >
      <UiIconButton
        aria-label={t("common.back")}
        className="shrink-0"
        onClick={onBack}
        shape="round"
        size="lg"
        variant="ghost"
      >
        <ArrowLeft className="h-4 w-4" />
      </UiIconButton>

      <UiButton
        aria-expanded={isConversationSwitcherOpen}
        aria-haspopup="dialog"
        aria-label={t("room.switch_conversation")}
        className="min-w-0 flex-1 justify-start gap-1.5 px-2 py-1 text-left"
        onClick={onOpenConversations}
        size="sm"
        title={conversationTitle}
        variant="ghost"
      >
        <div className="min-w-0 flex-1">
          <p className={cn(
            "truncate",
            getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
          )}>
            {primaryTitle}
          </p>
          {secondaryTitle ? (
            <p className={cn(
              "truncate",
              getUiTypographyClassName({ role: "caption", tone: "soft" }),
            )}>
              {secondaryTitle}
            </p>
          ) : null}
        </div>
        <ChevronDown className={cn(
          "h-3.5 w-3.5 shrink-0 text-(--text-muted) transition-transform duration-(--motion-duration-fast)",
          isConversationSwitcherOpen && "rotate-180",
        )} />
      </UiButton>

      <div className="flex shrink-0 items-center gap-0.5">
        {trailing}
      </div>
    </header>
  );
}
