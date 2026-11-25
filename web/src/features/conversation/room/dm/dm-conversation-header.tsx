// INPUT: DM 身份、会话目录、当前标签、导航命令与是否为单会话主窗口。
// OUTPUT: 使用标准主身份头像的 DM Header；主智能体主窗口不显示会话标签与历史菜单。
// POS: DM 顶栏装配；标签持久化与创建/关闭事务归 navigation owner。

"use client";

import { memo } from "react";

import { CONVERSATION_TOUR_ANCHORS } from "@/features/onboarding/tours/conversation-tour";
import { buildRoomHeaderTabs } from "@/features/conversation/room/surface/header/room-header-tabs";
import { RoomHistoryMenu } from "@/features/conversation/room/surface/history/room-history-menu";
import { useSidebarStore } from "@/store/sidebar";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/display/avatar";
import { RoomConversationTabs } from "@/features/navigation/conversation-tabs/room-conversation-tabs";
import type { FinalConversationReplacementHandler } from "@/features/navigation/conversation-tabs/final-conversation-replacement";
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
  onReplaceFinalConversation: FinalConversationReplacementHandler;
  onDeleteConversation: (conversationId: string) => Promise<string | null>;
  onSelectConversation: (conversationId: string) => void;
  onUpdateConversationTitle?: (conversationId: string, title: string) => Promise<void>;
  singleConversation?: boolean;
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
  singleConversation = false,
}: DmConversationHeaderProps) {
  const { t } = useI18n();
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const headerTitle = currentAgentName?.trim() || t("room.untitled_dm");
  const roomTabs = buildRoomHeaderTabs(t);

  return (
    <WorkspaceSurfaceHeader
      activeTab={activeTab}
      compactTabsLabel={t("room.panels")}
      leading={widePanelCollapsed ? undefined : (
        <UiAgentAvatar
          avatar={currentAgentAvatar}
          name={headerTitle}
          size="md"
        />
      )}
      leadingVariant={widePanelCollapsed ? undefined : "identity"}
      onChangeTab={onChangeTab}
      tabs={roomTabs}
      tabsLeading={singleConversation ? undefined : (
        <RoomConversationTabs
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
