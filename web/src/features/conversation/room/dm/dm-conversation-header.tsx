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
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}

const DmConversationHeaderView = memo(({
  conversation_id,
  conversations,
  current_agent_name,
  current_agent_avatar,
  todos,
  active_tab,
  on_replay_tour,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
}: DmConversationHeaderProps) => {
  const { t } = useI18n();
  const header_title = current_agent_name?.trim() || t("room.untitled_dm");
  const dm_tabs: {
    key: RoomSurfaceTabKey;
    label: string;
    icon: LucideIcon;
    anchor?: string;
  }[] = [
    { key: "history", label: t("room.history"), icon: History, anchor: CONVERSATION_TOUR_ANCHORS.tab_history },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree, anchor: CONVERSATION_TOUR_ANCHORS.tab_workspace },
    { key: "about", label: t("room.about"), icon: Info, anchor: CONVERSATION_TOUR_ANCHORS.tab_about },
  ];

  const conversation_tabs = (
    <WorkspaceConversationTabs
      conversations={conversations}
      conversation_id={conversation_id}
      on_create_conversation={on_create_conversation}
      on_select_conversation={on_select_conversation}
      tour_anchor={CONVERSATION_TOUR_ANCHORS.session_switcher}
    />
  );

  return (
    <WorkspaceSurfaceHeader
      active_tab={active_tab}
      badge="DM"
      density="compact"
      leading={<UiAgentAvatar avatar={current_agent_avatar} class_name="h-full w-full border-0 shadow-none" name={header_title} size="sm" />}
      on_change_tab={on_change_tab}
      tabs_leading={conversation_tabs}
      tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
      tabs={dm_tabs}
      title={header_title}
      trailing={on_replay_tour ? (
        <div className="flex items-center gap-2">
          <WorkspaceSurfaceToolbarAction onClick={on_replay_tour}>
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
