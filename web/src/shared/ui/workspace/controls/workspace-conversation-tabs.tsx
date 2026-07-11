"use client";

import { Plus } from "lucide-react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/lib/utils";
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
  "soft-scrollbar scrollbar-hide flex h-[30px] w-full min-w-0 items-center gap-0 overflow-x-auto rounded-[15px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_66%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] px-px py-px shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)]";

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
          className="relative ml-1 inline-flex h-6.5 w-[84px] shrink-0 items-center justify-start rounded-[13px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-panel-background)_76%,transparent)] pl-[22px] pr-2 text-left text-[11px] font-semibold leading-none text-(--text-default) shadow-[inset_0_1px_0_color-mix(in_srgb,var(--foreground)_5%,transparent)] transition-[background-color,border-color,color,box-shadow] duration-(--motion-duration-fast) ease-out hover:border-[color:color-mix(in_srgb,var(--success)_24%,var(--divider-subtle-color)_76%)] hover:bg-(--surface-interactive-hover-background) hover:text-(--success) hover:shadow-[inset_0_1px_0_color-mix(in_srgb,var(--success)_8%,transparent)] disabled:opacity-60"
          disabled={controller.isCreating}
          onClick={() => {
            void controller.createConversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          <Plus className={cn(
            "absolute left-[7px] top-1/2 h-3 w-3 -translate-y-1/2",
            controller.isCreating && "animate-spin",
          )} />
          <span className="min-w-0 truncate">{t("room.new_conversation")}</span>
        </button>
      ) : null}
    </nav>
  );
}
