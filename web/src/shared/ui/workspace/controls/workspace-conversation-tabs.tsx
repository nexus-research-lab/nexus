"use client";

import { Plus } from "lucide-react";
import type { ReactNode } from "react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConversationTabsScrollRail } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-scroll-rail";
import { useConversationTabsController } from "@/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-controller";
import { WorkspaceConversationTab } from "@/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab";
import { RoomConversationView } from "@/types/conversation/conversation";

interface WorkspaceConversationTabsProps {
  conversations: RoomConversationView[];
  conversationId: string | null;
  leadingControl?: ReactNode;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation?: (
    conversation: RoomConversationView,
    commitConversation: (conversationId: string) => boolean,
  ) => Promise<string | null>;
}

const TRACK_CLASS_NAME =
  "workspace-surface-header-session-tabs-track relative flex h-9 w-full min-w-0 items-center";

export function WorkspaceConversationTabs({
  conversations,
  conversationId,
  leadingControl,
  tourAnchor,
  onSelectConversation,
  onCloseConversation,
  onCreateConversation,
  onReplaceFinalConversation,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const controller = useConversationTabsController({
    conversations,
    conversationId,
    hasLeadingControl: Boolean(leadingControl),
    onCloseConversation,
    onCreateConversation,
    onReplaceFinalConversation,
    onSelectConversation,
  });

  return (
    <nav
      aria-label={t("room.session_tabs_label")}
      className={TRACK_CLASS_NAME}
      data-tour-anchor={tourAnchor}
      ref={controller.trackRef}
    >
      {leadingControl}

      <div className="workspace-surface-header-session-tabs-viewport-shell relative min-w-0 flex-1 self-stretch">
        <div
          className={cn(
            "workspace-surface-header-session-tabs-viewport scrollbar-hide flex h-full min-w-0 snap-x snap-proximity items-center gap-0.5 overflow-x-auto overflow-y-hidden overscroll-x-contain",
            controller.tabsScroll.isDragging ? "cursor-grabbing select-none" : "cursor-grab",
          )}
          onClickCapture={controller.tabsScroll.handleClickCapture}
          onPointerCancel={controller.tabsScroll.handlePointerCancel}
          onPointerDown={controller.tabsScroll.handlePointerDown}
          onPointerMove={controller.tabsScroll.handlePointerMove}
          onPointerUp={controller.tabsScroll.handlePointerUp}
          ref={controller.tabsScroll.viewportRef}
        >
          {controller.orderedConversations.map((conversation) => {
            const conversationId = conversation.conversation_id;
            const isActive = conversationId === controller.activeConversationId;

            return (
              <WorkspaceConversationTab
                canClose={controller.orderedConversations.length > 1 || Boolean(onReplaceFinalConversation)}
                closeLabel={t("room.close_conversation")}
                conversationId={conversationId}
                externalSessionLabel={getExternalSessionConversationLabel(conversation)}
                isActive={isActive}
                key={conversationId}
                onClose={() => controller.closeConversation(conversationId)}
                onSelect={() => controller.selectConversation(conversationId)}
                tabWidth={controller.tabWidths.get(conversationId)}
                title={conversation.title?.trim() || t("room.new_conversation")}
              />
            );
          })}
        </div>
        {controller.hasTabsOverflow ? (
          <ConversationTabsScrollRail
            ariaLabel={t("room.session_tabs_label")}
            metrics={controller.tabsScroll.metrics}
            onChange={controller.tabsScroll.setScrollLeft}
          />
        ) : null}
      </div>

      {onCreateConversation ? (
        <button
          aria-label={t("room.new_conversation")}
          className="workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-create relative inline-flex h-8 w-8 shrink-0 items-center justify-center leading-none transition-colors duration-(--motion-duration-fast) ease-out focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)] disabled:opacity-60"
          disabled={controller.isCreating}
          onClick={() => {
            void controller.createConversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          <Plus className={cn(
            "h-[18px] w-[18px] shrink-0",
            controller.isCreating && "animate-spin",
          )} />
        </button>
      ) : null}
    </nav>
  );
}
