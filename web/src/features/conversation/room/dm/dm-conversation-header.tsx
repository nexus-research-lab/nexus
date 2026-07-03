"use client";

import { memo } from "react";
import {
  Compass,
  FolderTree,
  History,
  Info,
  type LucideIcon,
} from "lucide-react";

import { UiAgentAvatar } from "@/shared/ui/avatar";
import {
  WorkspaceSurfaceHeader,
  WorkspaceTaskStrip,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceConversationTabs } from "@/shared/ui/workspace/controls/workspace-conversation-tabs";
import { RoomSurfaceTabKey } from "@/types/conversation/room-surface";
import { TodoItem } from "@/types/conversation/todo";
import { RoomConversationView } from "@/types/conversation/conversation";
import { CONVERSATION_TOUR_ANCHORS } from "../room-tour";

interface DmConversationHeaderProps {
  conversationId: string | null;
  conversations: RoomConversationView[];
  currentAgentName: string | null;
  currentAgentAvatar?: string | null;
  todos: TodoItem[];
  activeTab: RoomSurfaceTabKey;
  onReplayTour?: () => void;
  onChangeTab: (tab: RoomSurfaceTabKey) => void;
  onSelectConversation: (conversationId: string) => void;
  onCloseConversation: (conversationId: string) => Promise<void>;
  onCreateConversation?: (title?: string) => Promise<string | null>;
}

const DmConversationHeaderView = memo(({
  conversationId: conversationId,
  conversations,
  currentAgentName: currentAgentName,
  currentAgentAvatar: currentAgentAvatar,
  todos,
  activeTab: activeTab,
  onReplayTour: onReplayTour,
  onChangeTab: onChangeTab,
  onSelectConversation: onSelectConversation,
  onCloseConversation: onCloseConversation,
  onCreateConversation: onCreateConversation,
}: DmConversationHeaderProps) => {
  const { t } = useI18n();
  const headerTitle = currentAgentName?.trim() || t("room.untitled_dm");
  const dmTabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
    { key: "about", label: t("room.about"), icon: Info, anchor: CONVERSATION_TOUR_ANCHORS.tab_about },
  ];

  const conversationTabs = (
    <WorkspaceConversationTabs
      conversations={conversations}
      conversationId={conversationId}
      onCloseConversation={onCloseConversation}
      onCreateConversation={onCreateConversation}
      onSelectConversation={onSelectConversation}
      tourAnchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  return (
    <WorkspaceSurfaceHeader
      activeTab={activeTab}
      badge="DM"
      density="compact"
      leading={<UiAgentAvatar avatar={currentAgentAvatar} className="h-full w-full border-0 shadow-none" name={headerTitle} size="sm" />}
      onChangeTab={onChangeTab}
      tabsLeading={conversationTabs}
      tabsTrailing={<WorkspaceTaskStrip todos={todos} />}
      tabs={dmTabs}
      title={headerTitle}
      trailing={onReplayTour ? (
        <div className="flex items-center gap-2">
          <WorkspaceSurfaceToolbarAction onClick={onReplayTour}>
            <Compass className="h-3.5 w-3.5" />
            {t("common.view_guide")}
          </WorkspaceSurfaceToolbarAction>
        </div>
      ) : undefined}
    />
  );
});

DmConversationHeaderView.displayName = "DmConversationHeaderView";

export function DmConversationHeader(props: DmConversationHeaderProps) {
  return <DmConversationHeaderView {...props} />;
}
