"use client";

import { Check, ChevronDown, MessageSquareText } from "lucide-react";
import { useRef, useState } from "react";

import { formatRelativeTime } from "@/lib/format/relative-time";
import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiActionMenu } from "@/shared/ui/menu/action-menu";
import type { RoomConversationView } from "@/types/conversation/conversation";

interface ConversationTabsOverviewProps {
  activeConversationId: string | null;
  conversations: RoomConversationView[];
  onSelectConversation: (conversationId: string) => void;
}

export function ConversationTabsOverview({
  activeConversationId,
  conversations,
  onSelectConversation,
}: ConversationTabsOverviewProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);

  if (conversations.length <= 1) {
    return null;
  }

  return (
    <>
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("room.switch_conversation")}
        className={cn(
          "inline-flex h-8 w-10 shrink-0 items-center justify-center rounded-[9px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_72%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_38%,transparent)] text-(--icon-default) transition-[background-color,border-color,color] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color)_82%)] hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
          isOpen && "border-[color:color-mix(in_srgb,var(--primary)_28%,var(--divider-subtle-color)_72%)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] text-(--primary)",
        )}
        onClick={() => setIsOpen((current) => !current)}
        title={t("room.switch_conversation")}
        type="button"
      >
        <ChevronDown
          className={cn(
            "h-3.5 w-3.5 transition-transform duration-(--motion-duration-fast)",
            isOpen && "rotate-180",
          )}
        />
      </button>

      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("room.switch_conversation")}
        className="bg-(--surface-panel-background)"
        isOpen={isOpen}
        items={conversations.map((conversation) => {
          const isActive = conversation.conversation_id === activeConversationId;
          const externalSessionLabel = getExternalSessionConversationLabel(conversation);
          return {
            active: isActive,
            description: [
              formatRelativeTime(conversation.last_activity_at),
              externalSessionLabel ? `IM ${externalSessionLabel}` : null,
            ].filter(Boolean).join(" · "),
            icon: <MessageSquareText className="h-4 w-4 text-(--icon-muted)" />,
            label: conversation.title?.trim() || t("room.untitled_conversation"),
            tone: isActive ? "primary" as const : "default" as const,
            trailing: isActive ? (
              <span className="inline-flex items-center gap-1 text-[10px] font-semibold text-(--primary)">
                <Check className="h-3.5 w-3.5" />
                {t("room.current_conversation")}
              </span>
            ) : undefined,
            value: conversation.conversation_id,
          };
        })}
        minWidth={300}
        onClose={() => setIsOpen(false)}
        onSelect={onSelectConversation}
        placement="bottom"
      />
    </>
  );
}
