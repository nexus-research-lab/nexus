"use client";

import { memo } from "react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { RoomHeaderGuideMenu } from "@/features/conversation/room/surface/header/room-header-guide-menu";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";
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
  onCloseActiveTab: () => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
  onReplayTour?: () => void;
  onSelectConversation: (conversationId: string) => void;
}

export const DmConversationHeader = memo(function DmConversationHeader({
  activeTab,
  conversationId,
  conversations,
  currentAgentAvatar,
  currentAgentName,
  onChangeTab,
  onCloseActiveTab,
  onCloseConversation,
  onCreateConversation,
  onReplayTour,
  onSelectConversation,
}: DmConversationHeaderProps) {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const headerTitle = currentAgentName?.trim() || t("room.untitled_dm");

  return (
    <WorkspaceSurfaceHeader
      activeTab={activeTab}
      dismissActiveTabLabel={t("common.close")}
      leading={(
        <UiAgentAvatar
          avatar={currentAgentAvatar}
          className="h-full w-full border-0 shadow-none"
          name={headerTitle}
          size="sm"
        />
      )}
      onChangeTab={onChangeTab}
      onDismissActiveTab={onCloseActiveTab}
      tabs={buildRoomHeaderTabs(t)}
      tabsLeading={(
        <WorkspaceConversationTabs
          conversationId={conversationId}
          conversations={conversations}
          onCloseConversation={onCloseConversation}
          onCreateConversation={onCreateConversation}
          onSelectConversation={onSelectConversation}
          tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
        />
      )}
      title={widePanelCollapsed ? headerTitle : undefined}
      trailing={onReplayTour ? (
        <RoomHeaderGuideMenu onReplayTour={onReplayTour} />
      ) : undefined}
    />
  );
});
