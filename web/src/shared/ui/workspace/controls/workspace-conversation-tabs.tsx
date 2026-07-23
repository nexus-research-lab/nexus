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

      <div className="workspace-surface-header-session-tabs-actions flex shrink-0 items-center">
        {controller.tabsScroll.hasOverflow || onCreateConversation ? (
          <div className="inline-flex h-8 shrink-0 items-stretch overflow-hidden rounded-full border border-[color:color-mix(in_srgb,var(--primary)_18%,var(--divider-subtle-color)_82%)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_58%,transparent)] shadow-[0_1px_2px_rgba(15,23,42,0.06)] transition-[background-color,border-color,box-shadow] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--primary)_30%,var(--divider-subtle-color)_70%)] hover:bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_76%,transparent)] hover:shadow-[0_2px_8px_rgba(15,23,42,0.08)]">
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
                className="relative inline-flex h-full w-10 shrink-0 items-center justify-center bg-transparent leading-none text-(--primary) transition-[background-color,color] duration-(--motion-duration-fast) ease-out hover:bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)] disabled:opacity-60"
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
              </button>
            ) : null}
          </div>
        ) : null}
      </div>
    </nav>
  );
}
