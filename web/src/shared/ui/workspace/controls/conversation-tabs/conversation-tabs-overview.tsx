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
          "workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-overview inline-flex h-full w-9 shrink-0 items-center justify-center rounded-l-[var(--workspace-session-tray-radius)] bg-transparent text-(--icon-default) transition-colors duration-(--motion-duration-fast) hover:text-(--text-strong) focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)]",
          isOpen && "text-(--text-strong)",
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
        className="conversation-tabs-overview-menu"
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
            tone: "default" as const,
            trailing: isActive ? (
              <span className="inline-flex items-center gap-1 text-2xs font-semibold text-(--primary)">
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
