"use client";

import { Plus } from "lucide-react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useConversationTabsController } from "@/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-controller";
import { WorkspaceConversationTab } from "@/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab";
import { RoomConversationView } from "@/types/conversation/conversation";

interface WorkspaceConversationTabsProps {
  conversations: RoomConversationView[];
  conversationId: string | null;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
}

const TRACK_CLASS_NAME =
  "soft-scrollbar scrollbar-hide flex h-8 w-full min-w-0 items-center gap-0 overflow-x-auto border-b border-(--divider-subtle-color) px-0 py-0";

export function WorkspaceConversationTabs({
  conversations,
  conversationId,
  tourAnchor,
  onSelectConversation,
  onCloseConversation,
  onCreateConversation,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const controller = useConversationTabsController({
    conversations,
    conversationId,
    onCloseConversation,
    onCreateConversation,
    onSelectConversation,
  });

  return (
    <nav
      aria-label={t("room.session_tabs_label")}
      className={TRACK_CLASS_NAME}
      data-tour-anchor={tourAnchor}
      ref={controller.trackRef}
    >
      {controller.orderedConversations.map((conversation, index) => {
        const conversationId = conversation.conversation_id;
        const previousConversation = controller.orderedConversations[index - 1];
        const isActive = conversationId === controller.activeConversationId;
        const isHovered = conversationId === controller.hoveredConversationId;
        const isPreviousHighlighted = previousConversation && (
          previousConversation.conversation_id === controller.activeConversationId ||
          previousConversation.conversation_id === controller.hoveredConversationId
        );

        return (
          <WorkspaceConversationTab
            canClose={controller.orderedConversations.length > 1}
            closeLabel={t("room.close_conversation")}
            externalSessionLabel={getExternalSessionConversationLabel(conversation)}
            isActive={isActive}
            key={conversationId}
            onClose={() => controller.closeConversation(conversationId)}
            onHoverChange={(hovered) => {
              controller.setConversationHovered(conversationId, hovered);
            }}
            onPreview={() => controller.previewConversation(conversationId)}
            onSelect={() => controller.selectConversation(conversationId)}
            showSeparator={index > 0 && !isActive && !isHovered && !isPreviousHighlighted}
            tabWidth={controller.tabWidths.get(conversationId)}
            title={conversation.title?.trim() || t("room.untitled_conversation")}
          />
        );
      })}

      {onCreateConversation ? (
        <button
          aria-label={t("room.new_conversation")}
          className="relative ml-1 inline-flex h-7 min-w-[76px] shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-[6px] border border-[color:color-mix(in_srgb,var(--primary)_22%,var(--divider-subtle-color)_78%)] bg-[color:color-mix(in_srgb,var(--primary)_7%,transparent)] px-2.5 text-left text-[11px] font-medium leading-none text-(--primary) transition-[background-color,border-color,color] duration-(--motion-duration-fast) ease-out hover:border-[color:color-mix(in_srgb,var(--primary)_36%,var(--divider-subtle-color)_64%)] hover:bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] hover:text-(--primary) disabled:opacity-60"
          disabled={controller.isCreating}
          onClick={() => {
            void controller.createConversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          <Plus className={cn(
            "h-3.5 w-3.5 shrink-0",
            controller.isCreating && "animate-spin",
          )} />
          <span className="min-w-0 truncate">{t("room.new_conversation")}</span>
        </button>
      ) : null}
    </nav>
  );
}
