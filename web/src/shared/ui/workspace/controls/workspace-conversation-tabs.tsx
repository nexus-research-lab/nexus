"use client";

import { Plus } from "lucide-react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConversationTabsOverview } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-overview";
import { ConversationTabsScrollRail } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-scroll-rail";
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
  "workspace-surface-header-session-tabs-track flex h-9 w-full min-w-0 items-center gap-0.5 overflow-hidden px-0.5 py-0.5";

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
      <div className="workspace-surface-header-session-tabs-viewport-shell relative min-w-0 flex-1 self-stretch">
        <div
          className={cn(
            "workspace-surface-header-session-tabs-viewport scrollbar-hide flex h-full min-w-0 items-center gap-0.5 overflow-x-auto overflow-y-hidden overscroll-x-contain",
            controller.tabsScroll.isDragging ? "cursor-grabbing select-none" : "cursor-grab",
          )}
          onClickCapture={controller.tabsScroll.handleClickCapture}
          onPointerCancel={controller.tabsScroll.handlePointerCancel}
          onPointerDown={controller.tabsScroll.handlePointerDown}
          onPointerMove={controller.tabsScroll.handlePointerMove}
          onPointerUp={controller.tabsScroll.handlePointerUp}
          onWheel={controller.tabsScroll.handleWheel}
          ref={controller.tabsScroll.viewportRef}
        >
          {controller.orderedConversations.map((conversation, index) => {
            const conversationId = conversation.conversation_id;
            const previousConversation = controller.orderedConversations[index - 1];
            const isActive = conversationId === controller.activeConversationId;

            return (
              <WorkspaceConversationTab
                canClose={controller.orderedConversations.length > 1}
                closeLabel={t("room.close_conversation")}
                conversationId={conversationId}
                externalSessionLabel={getExternalSessionConversationLabel(conversation)}
                isActive={isActive}
                key={conversationId}
                onClose={() => controller.closeConversation(conversationId)}
                onSelect={() => controller.selectConversation(conversationId)}
                showSeparator={index > 0
                  && !isActive
                  && previousConversation?.conversation_id !== controller.activeConversationId}
                tabWidth={controller.tabWidths.get(conversationId)}
                title={conversation.title?.trim() || t("room.untitled_conversation")}
              />
            );
          })}
        </div>
        {controller.tabsScroll.hasOverflow ? (
          <ConversationTabsScrollRail
            ariaLabel={t("room.session_tabs_label")}
            metrics={controller.tabsScroll.metrics}
            onChange={controller.tabsScroll.setScrollLeft}
          />
        ) : null}
      </div>

      <div className="workspace-surface-header-session-tabs-actions flex shrink-0 items-center gap-0.5">
        {controller.tabsScroll.hasOverflow ? (
          <ConversationTabsOverview
            activeConversationId={controller.activeConversationId}
            conversations={controller.recentConversations}
            onSelectConversation={controller.selectConversation}
          />
        ) : null}

        {onCreateConversation ? (
          <button
            aria-label={t("room.new_conversation")}
            className="relative inline-flex h-8 min-w-[76px] shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-[9px] border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color)_82%)] bg-[color:color-mix(in_srgb,var(--primary)_6%,transparent)] px-2.5 text-left text-[11px] font-semibold leading-none text-(--primary) transition-[background-color,border-color,color,box-shadow] duration-(--motion-duration-fast) ease-out hover:border-[color:color-mix(in_srgb,var(--primary)_34%,var(--divider-subtle-color)_66%)] hover:bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] hover:shadow-[0_2px_8px_color-mix(in_srgb,var(--primary)_8%,transparent)] disabled:opacity-60"
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
            <span className="workspace-conversation-create-label-full min-w-0 truncate">
              {t("room.new_conversation")}
            </span>
            <span aria-hidden="true" className="workspace-conversation-create-label-compact min-w-0 truncate">
              {t("room.new_conversation_short")}
            </span>
          </button>
        ) : null}
      </div>
    </nav>
  );
}
