"use client";

import { memo } from "react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";
import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";
import { useSidebarStore } from "@/store/sidebar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { RoomConversationView } from "@/types/conversation/conversation";
import type { RoomSurfaceTabKey } from "@/features/conversation/room/surface/header/room-header-tabs";

interface DmConversationHeaderProps {
  activeTab: RoomSurfaceTabKey;
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentAgentAvatar?: string | null;
  currentAgentName: string | null;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation: (title?: string) => Promise<string | null>;
  onReplaceFinalConversation: (
    conversation: RoomConversationView,
    commitConversation: (conversationId: string) => boolean,
  ) => Promise<string | null>;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
}

export const DmConversationHeader = memo(function DmConversationHeader({
  activeTab,
  conversationId,
  conversations,
  currentAgentAvatar,
  currentAgentName,
  onChangeTab,
  onCloseConversation,
  onCreateConversation,
  onReplaceFinalConversation,
  onDeleteConversation,
  onSelectConversation,
  onUpdateConversationTitle,
}: DmConversationHeaderProps) {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const headerTitle = currentAgentName?.trim() || t("room.untitled_dm");
  const roomTabs = buildRoomHeaderTabs(t);

  return (
    <WorkspaceSurfaceHeader
      activeTab={activeTab}
      compactTabsLabel={t("room.panels")}
      leading={(
        <UiAgentAvatar
          avatar={currentAgentAvatar}
          className="h-full w-full border-0 shadow-none"
          name={headerTitle}
          size="sm"
        />
      )}
      leadingClassName="h-10 w-10"
      leadingVariant="identity"
      onChangeTab={onChangeTab}
      tabs={roomTabs}
      tabsLeading={(
        <WorkspaceConversationTabs
          conversationId={conversationId}
          conversations={conversations}
          leadingControl={(
            <RoomHistoryMenu
              conversationId={conversationId}
              conversations={conversations}
              onCreateConversation={onCreateConversation}
              onDeleteConversation={onDeleteConversation}
              onSelectConversation={onSelectConversation}
              onUpdateConversationTitle={onUpdateConversationTitle}
              triggerVariant="session"
            />
          )}
          onCloseConversation={onCloseConversation}
          onCreateConversation={onCreateConversation}
          onReplaceFinalConversation={onReplaceFinalConversation}
          onSelectConversation={onSelectConversation}
          tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
        />
      )}
      title={widePanelCollapsed ? headerTitle : undefined}
    />
  );
});
