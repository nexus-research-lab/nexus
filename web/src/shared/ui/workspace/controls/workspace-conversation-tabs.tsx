/**
 * INPUT: 受控标签展示项、活动身份、busy 状态及选择/关闭/固定/创建命令。
 * OUTPUT: 保持既有几何与滚动的历史/标签/创建导航带。
 * POS: 共享标签 DOM 与交互视图；不认识 Room Store、会话协议或持久化事务。
 */
"use client";

import { LoaderCircle, Plus } from "lucide-react";
import type { ReactNode } from "react";

import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConversationTabsScrollRail } from "@/shared/ui/workspace/controls/conversation-tabs/conversation-tabs-scroll-rail";
import { useConversationTabsLayout } from "@/shared/ui/workspace/controls/conversation-tabs/use-conversation-tabs-layout";
import { WorkspaceConversationTab } from "@/shared/ui/workspace/controls/conversation-tabs/workspace-conversation-tab";

export interface WorkspaceConversationTabItem {
  id: string;
  title: string;
  canClose: boolean;
  canPin?: boolean;
  isPinned?: boolean;
  externalSessionLabel?: string | null;
}

interface WorkspaceConversationTabsProps {
  tabs: readonly WorkspaceConversationTabItem[];
  activeConversationId: string | null;
  isCreating?: boolean;
  leadingControl?: ReactNode;
  tourAnchor?: string;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => void;
  onCreateConversation?: () => void;
  onTogglePin?: (conversationId: string) => void;
}

const TRACK_CLASS_NAME =
  "workspace-surface-header-session-tabs-track relative flex h-9 w-full min-w-0 items-center";

export function WorkspaceConversationTabs({
  tabs,
  activeConversationId,
  isCreating = false,
  leadingControl,
  tourAnchor,
  onSelectConversation,
  onCloseConversation,
  onCreateConversation,
  onTogglePin,
}: WorkspaceConversationTabsProps) {
  const { t } = useI18n();
  const controller = useConversationTabsLayout({
    tabs,
    activeConversationId,
    hasLeadingControl: Boolean(leadingControl),
    hasCreateButton: Boolean(onCreateConversation),
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
          {tabs.map((tab) => {
            const conversationId = tab.id;
            const isActive = conversationId === activeConversationId;

            return (
              <WorkspaceConversationTab
                canClose={tab.canClose}
                canPin={Boolean(tab.canPin && onTogglePin)}
                closeLabel={t("room.close_conversation")}
                conversationId={conversationId}
                externalSessionLabel={tab.externalSessionLabel ?? null}
                isActive={isActive}
                isPinned={Boolean(tab.isPinned)}
                key={conversationId}
                onClose={() => onCloseConversation(conversationId)}
                onSelect={() => onSelectConversation(conversationId)}
                onTogglePin={() => onTogglePin?.(conversationId)}
                pinLabel={t(tab.isPinned
                  ? "room.unpin_conversation"
                  : "room.pin_conversation")}
                tabWidth={controller.tabWidths.get(conversationId)}
                title={tab.title}
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
        <UiIconButton
          aria-busy={isCreating}
          aria-label={t("room.new_conversation")}
          className="workspace-surface-header-session-tabs-edge-action workspace-surface-header-session-tabs-create relative shrink-0 leading-none focus-visible:z-10"
          disabled={isCreating}
          onClick={() => {
            onCreateConversation();
          }}
          size="md"
          tooltip={t("room.new_conversation")}
          variant="ghost"
        >
          {isCreating ? (
            <LoaderCircle
              aria-hidden
              className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
            />
          ) : (
            <Plus aria-hidden className="h-[18px] w-[18px] shrink-0" />
          )}
        </UiIconButton>
      ) : null}
    </nav>
  );
}
