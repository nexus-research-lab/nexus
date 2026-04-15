"use client";

import { memo } from "react";
import {
  Bot,
  FolderTree,
  History,
  Info,
  MessageSquare,
} from "lucide-react";

import { get_icon_avatar_src } from "@/lib/utils";
import {
  WorkspaceSurfaceHeader,
  WorkspaceTaskStrip,
} from "@/shared/ui/workspace/workspace-surface-header";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceConversationSwitcher } from "@/shared/ui/workspace/workspace-conversation-switcher";
import { WorkspaceStatusBadge } from "@/shared/ui/workspace/workspace-status-badge";
import { RoomSurfaceTabKey } from "@/types/room-surface";
import { TodoItem } from "@/types/todo";
import { RoomConversationView } from "@/types/conversation";

interface DmConversationHeaderProps {
  conversation_id: string | null;
  conversations: RoomConversationView[];
  current_agent_name: string | null;
  current_agent_avatar?: string | null;
  is_loading: boolean;
  todos: TodoItem[];
  active_tab: RoomSurfaceTabKey;
  on_change_tab: (tab: RoomSurfaceTabKey) => void;
  on_select_conversation: (conversation_id: string) => void;
  on_create_conversation?: (title?: string) => Promise<string | null>;
}

const DmConversationHeaderView = memo(({
  conversation_id,
  conversations,
  current_agent_name,
  current_agent_avatar,
  is_loading,
  todos,
  active_tab,
  on_change_tab,
  on_select_conversation,
  on_create_conversation,
}: DmConversationHeaderProps) => {
  const { t } = useI18n();
  const header_title = current_agent_name?.trim() || t("room.untitled_dm");
  const current_agent_avatar_src = get_icon_avatar_src(current_agent_avatar);
  const dm_tabs: { key: RoomSurfaceTabKey; label: string; icon: typeof MessageSquare }[] = [
    { key: "chat", label: t("room.chat"), icon: MessageSquare },
    { key: "history", label: t("room.history"), icon: History },
    { key: "workspace", label: t("room.workspace"), icon: FolderTree },
    { key: "about", label: t("room.about"), icon: Info },
  ];

  const title_trailing = (
    <WorkspaceConversationSwitcher
      conversations={conversations}
      conversation_id={conversation_id}
      density="compact"
      on_select_conversation={on_select_conversation}
      on_create_conversation={on_create_conversation}
      on_view_history={() => on_change_tab("history")}
    />
  );

  const trailing = is_loading ? (
    <WorkspaceStatusBadge
      icon={<span className="text-current">●</span>}
      label={t("status.replying")}
      size="compact"
      tone="running"
    />
  ) : null;

  return (
    <WorkspaceSurfaceHeader
      active_tab={active_tab}
      badge="DM"
      density="compact"
      leading={current_agent_avatar_src ? (
        <img
          alt={header_title}
          className="h-full w-full rounded-full object-cover"
          src={current_agent_avatar_src}
        />
      ) : (
        <Bot size={14} className="text-(--icon-default)" />
      )}
      on_change_tab={on_change_tab}
      tabs_trailing={<WorkspaceTaskStrip todos={todos} />}
      tabs={dm_tabs}
      title={header_title}
      title_trailing={title_trailing}
      trailing={trailing}
    />
  );
});

DmConversationHeaderView.displayName = "DmConversationHeaderView";

export function DmConversationHeader(props: DmConversationHeaderProps) {
  return <DmConversationHeaderView {...props} />;
}
