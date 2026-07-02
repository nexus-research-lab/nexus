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
  conversation_id: string | null;
  conversations: RoomConversationView[];
  current_agent_name: string | null;
  current_agent_avatar?: string | null;
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_replay_tour?: () => void;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversationId: string) => void;
  on_close_conversation: (conversationId: string) => Promise<void>;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}

const DmConversationHeaderView = memo(({
  conversation_id: conversationId,
  conversations,
  current_agent_name: currentAgentName,
  current_agent_avatar: currentAgentAvatar,
  todos,
  active_tab: activeTab,
  on_replay_tour: onReplayTour,
  on_change_tab: onChangeTab,
  on_select_conversation: onSelectConversation,
  on_close_conversation: onCloseConversation,
  on_create_conversation: onCreateConversation,
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
      conversation_id={conversationId}
      on_close_conversation={onCloseConversation}
      on_create_conversation={onCreateConversation}
      on_select_conversation={onSelectConversation}
      tour_anchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  return (
    <WorkspaceSurfaceHeader
      active_tab={activeTab}
      badge="DM"
      density="compact"
      leading={<UiAgentAvatar avatar={currentAgentAvatar} class_name="h-full w-full border-0 shadow-none" name={headerTitle} size="sm" />}
      on_change_tab={onChangeTab}
      tabs_leading={conversationTabs}
      tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
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
