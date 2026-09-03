/**
 * INPUT: Room 会话集合、当前选择、标签事务回调与可选固定能力。
 * OUTPUT: 历史/标签/创建导航带，以及与主侧栏同步的固定图钉动作。
 * POS: Workspace 会话标签编排层；集合事务归控制器，单项样式归 tab 视图。
 */
"use client";

import { LoaderCircle, Plus } from "lucide-react";
import type { ReactNode } from "react";

import { getExternalSessionConversationLabel } from "@/lib/conversation/external-session";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConversationTabsScrollRail } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-scroll-rail";
import type { FinalConversationReplacementHandler } from "@/shared/ui/workspace/controls/conversation-tabs/final-conversation-replacement";
import { useConversationTabsController } from "@/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-controller";
import { WorkspaceConversationTab } from "@/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab";
import { useRoomNavigationStore } from "@/store/room-navigation";
import { RoomConversationView } from "@/types/conversation/conversation";

interface WorkspaceConversationTabsProps {
  conversations: RoomConversationView[];
  conversationId: string | null;
  leadingControl?: ReactNode;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation?: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation?: FinalConversationReplacementHandler;
  pinningEnabled?: boolean;
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
  pinningEnabled = true,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const pinnedConversations = useRoomNavigationStore(
    (state) => state.pinned_conversations,
  );
  const togglePinnedConversation = useRoomNavigationStore(
    (state) => state.toggle_pinned_conversation,
  );
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
            const title = conversation.title?.trim() || t("room.new_conversation");
            const canPin = pinningEnabled && Boolean(
              conversation.room_id.trim() && conversationId.trim(),
            );
            const isPinned = pinnedConversations.some((item) => (
              item.room_id === conversation.room_id
              && item.conversation_id === conversationId
            ));

            return (
              <WorkspaceConversationTab
                canClose={controller.orderedConversations.length > 1 || Boolean(onReplaceFinalConversation)}
                canPin={canPin}
                closeLabel={t("room.close_conversation")}
                conversationId={conversationId}
                externalSessionLabel={getExternalSessionConversationLabel(conversation)}
                isActive={isActive}
                isPinned={isPinned}
                key={conversationId}
                onClose={() => controller.closeConversation(conversationId)}
                onSelect={() => controller.selectConversation(conversationId)}
                onTogglePin={() => togglePinnedConversation({
                  conversation_id: conversationId,
                  room_id: conversation.room_id,
                  session_key: conversation.session_key,
                  title,
                })}
                pinLabel={t(isPinned
                  ? "room.unpin_conversation"
                  : "room.pin_conversation")}
                tabWidth={controller.tabWidths.get(conversationId)}
                title={title}
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
          aria-busy={controller.isCreating}
          aria-label={t("room.new_conversation")}
          className="workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-create relative inline-flex h-8 w-8 shrink-0 items-center justify-center leading-none transition-colors duration-(--motion-duration-fast) ease-out focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_42%,transparent)] disabled:opacity-60"
          disabled={controller.isCreating}
          onClick={() => {
            void controller.createConversation();
          }}
          title={t("room.new_conversation")}
          type="button"
        >
          {controller.isCreating ? (
            <LoaderCircle
              aria-hidden
              className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
            />
          ) : (
            <Plus aria-hidden className="h-[18px] w-[18px] shrink-0" />
          )}
        </button>
      ) : null}
    </nav>
  );
}
