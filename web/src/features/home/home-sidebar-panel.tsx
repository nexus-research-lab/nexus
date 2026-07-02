/**
 * 聊天式侧边栏内容。
 *
 * 左侧面板从导航树收敛为三个真实工作入口：
 * - 聊天：统一承载 Room 与 DM。
 * - 联系人：管理 Agent，并提供发起 DM 的快捷动作。
 * - 能力：由侧边栏顶层 Tab 承载，不再混在聊天列表里。
 */

import {
  MessageSquarePlus,
  Plus,
  UserPlus,
  Users2,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { CreateRoomDialog } from "@/features/conversation/room/members/create-room-dialog";
import { create_room, delete_room } from "@/lib/api/room-api";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import {
  build_chat_notification_target_key,
  get_active_chat_target_from_path,
} from "./chat-notification-target";
import {
  build_conversation_items,
  build_sidebar_item_notification_key,
  get_sidebar_item_unread_state,
  is_active_sidebar_chat_item,
  is_main_agent_dm_room,
  normalize_query,
  type SidebarConversationItem,
} from "./home-sidebar-conversation-model";
import { useSidebarDirectory } from "./home-sidebar-directory";
import {
  ContactRow,
  ConversationRow,
  SidebarListLoadingRows,
  SidebarSearchField,
} from "./home-sidebar-list-rows";

interface DeleteTarget {
  id: string;
  name: string;
  room_type: "room" | "dm";
}

export const ChatSidebarPanelContent = memo(function ChatSidebarPanelContent() {
  const { t } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const activeItemId = useSidebarStore((s) => s.active_panel_item_id);
  const setActiveItem = useSidebarStore((s) => s.set_active_panel_item);
  const chatUnreadCounts = useSidebarStore((s) => s.chat_unread_counts);
  const chatUnreadTargets = useSidebarStore((s) => s.chat_unread_targets);
  const chatUnreadTimestamps = useSidebarStore((s) => s.chat_unread_timestamps);
  const clearChatNotificationsForTarget = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const clearChatNotificationsForRoom = useSidebarStore(
    (s) => s.clear_chat_notifications_for_room,
  );
  const setNexusRoomId = useSidebarStore((s) => s.set_nexus_room_id);
  const agentRuntimeStatuses = useAgentStore((s) => s.agent_runtime_statuses);
  const { agents, conversations, is_loading: isLoading, refresh_directory: refreshDirectory, rooms } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [isCreateRoomOpen, setIsCreateRoomOpen] = useState(false);
  const [isCreatingRoom, setIsCreatingRoom] = useState(false);
  const untitledRoomLabel = t("home.untitled_room");
  const hasAgents = agents.length > 0;

  const nexusDmRoom = useMemo(
    () => rooms.find((room) => is_main_agent_dm_room(room)) ?? null,
    [rooms],
  );
  const activeChatTarget = useMemo(
    () => get_active_chat_target_from_path(location.pathname),
    [location.pathname],
  );

  useEffect(() => {
    setNexusRoomId(nexusDmRoom?.id ?? null);
  }, [nexusDmRoom, setNexusRoomId]);

  const rawItems = useMemo(
    () => build_conversation_items({
      agents,
      agent_runtime_statuses: agentRuntimeStatuses,
      conversations,
      format_running_tasks_summary: (count) => t("sidebar.running_tasks_summary", { count }),
      rooms,
      untitled_room_label: untitledRoomLabel,
    }).map((item) => {
      const notificationKey = build_sidebar_item_notification_key(item);
      const unreadState = get_sidebar_item_unread_state({
        chat_unread_counts: chatUnreadCounts,
        chat_unread_targets: chatUnreadTargets,
        chat_unread_timestamps: chatUnreadTimestamps,
        notification_key: notificationKey,
        room_id: item.room_id,
        session_key: item.session_key,
      });
      return {
        ...item,
        notification_key: notificationKey,
        ...unreadState,
      };
    }),
    [
      agents,
      agentRuntimeStatuses,
      chatUnreadCounts,
      chatUnreadTargets,
      chatUnreadTimestamps,
      conversations,
      rooms,
      t,
      untitledRoomLabel,
    ],
  );
  const items = useMemo(
    () => rawItems.map((item) => {
      const visibleUnreadState = is_active_sidebar_chat_item(item, activeChatTarget)
        ? {
          unread_conversation_id: null,
          unread_count: 0,
          unread_target_key: null,
        }
        : {
          unread_conversation_id: item.unread_conversation_id ?? null,
          unread_count: item.unread_count ?? 0,
          unread_target_key: item.unread_target_key ?? null,
        };
      return {
        ...item,
        ...visibleUnreadState,
      };
    }),
    [activeChatTarget, rawItems],
  );

  const filteredItems = useMemo(() => {
    const normalizedQuery = normalize_query(query);
    if (!normalizedQuery) {
      return items;
    }
    return items.filter((item) => {
      const memberNames = item.members.map((member) => member.name).join(" ");
      return `${item.title} ${item.summary} ${memberNames}`.toLowerCase().includes(normalizedQuery);
    });
  }, [items, query]);

  const navigateToRoom = useCallback(async (item: SidebarConversationItem) => {
    const routeRoomId = item.route_room_id ?? item.room_id;
    if (!routeRoomId) {
      return;
    }
    const targetConversationId = item.unread_conversation_id || item.conversation_id;
    if (item.room_id) {
      clearChatNotificationsForRoom(item.room_id);
    }
    clearChatNotificationsForTarget(item.unread_target_key || item.notification_key);
    setActiveItem(item.id);
    if (targetConversationId) {
      navigate(AppRouteBuilders.room_conversation(routeRoomId, targetConversationId));
      return;
    }
    navigate(AppRouteBuilders.room(routeRoomId));
  }, [
    clearChatNotificationsForRoom,
    clearChatNotificationsForTarget,
    navigate,
    setActiveItem,
  ]);

  const handleCreateRoom = useCallback(() => {
    setIsCreateRoomOpen(true);
  }, []);

  const handleConfirmCreateRoom = useCallback(async (
    agentIds: string[],
    name: string,
    avatar?: string,
    skillNames?: string[],
    hostAgentId?: string | null,
    hostAutoReplyEnabled?: boolean,
    privateMessagesEnabled?: boolean,
  ) => {
    setIsCreatingRoom(true);
    try {
      const context = await create_room({
        agent_ids: agentIds,
        name,
        avatar,
        skill_names: skillNames,
        host_agent_id: hostAgentId,
        host_auto_reply_enabled: hostAutoReplyEnabled,
        private_messages_enabled: privateMessagesEnabled,
      });
      setIsCreateRoomOpen(false);
      refreshDirectory();
      navigate(AppRouteBuilders.room(context.room.id));
    } finally {
      setIsCreatingRoom(false);
    }
  }, [navigate, refreshDirectory]);

  const handleDeleteRoom = useCallback(async (target: DeleteTarget) => {
    const deletedRoomId = target.id;
    await delete_room(deletedRoomId);
    if (activeItemId === deletedRoomId) {
      setActiveItem(null);
    }
    refreshDirectory();
  }, [activeItemId, refreshDirectory, setActiveItem]);

  const handleConfirmDeleteRoom = useCallback(() => {
    const target = deleteTarget;
    if (!target) {
      return;
    }

    setDeleteTarget(null);
    void handleDeleteRoom(target).catch((error) => {
      console.error("[Sidebar] Failed to delete room", error);
      refreshDirectory();
    });
  }, [deleteTarget, handleDeleteRoom, refreshDirectory]);

  const emptyDescription = hasAgents
    ? t("home.rooms_empty_description")
    : t("home.rooms_empty_no_agents_description");
  const emptyAction = hasAgents
    ? t("home.rooms_empty_action")
    : t("home.rooms_empty_no_agents_action");
  const handleEmptyAction = hasAgents
    ? handleCreateRoom
    : () => navigate(AppRouteBuilders.contacts());

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.chat_list}>
      <SidebarSearchField
        action={(
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={handleCreateRoom}
            title={t("home.create_room")}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
        )}
        on_change={setQuery}
        placeholder={t("sidebar.search_conversations")}
        value={query}
      />

      {isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
          {filteredItems.length > 0 ? (
            filteredItems.map((item) => (
              <ConversationRow
                is_active={activeItemId === item.id || (item.room_id ? activeItemId === item.room_id : false)}
                item={item}
                key={item.id}
                on_click={() => {
                  void navigateToRoom(item);
                }}
                on_delete={item.can_delete && item.room_id ? () => setDeleteTarget({
                  id: item.room_id ?? item.id,
                  name: item.title,
                  room_type: item.kind,
                }) : undefined}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              action_label={emptyAction}
              description={emptyDescription}
              icon={MessageSquarePlus}
              on_action={handleEmptyAction}
              title={query ? t("sidebar.no_matching_conversations") : t("home.rooms_empty_title")}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        confirm_text={t("common.delete")}
        is_open={deleteTarget !== null}
        message={t("home.delete_message", { name: deleteTarget?.name ?? "" })}
        on_cancel={() => setDeleteTarget(null)}
        on_confirm={handleConfirmDeleteRoom}
        title={t("home.delete_confirm")}
        variant="danger"
      />

      <CreateRoomDialog
        agents={agents.map((agent) => ({
          agent_id: agent.id,
          name: agent.name,
          avatar: agent.avatar,
        }))}
        is_creating={isCreatingRoom}
        is_open={isCreateRoomOpen}
        on_cancel={() => setIsCreateRoomOpen(false)}
        on_confirm={(ids, name, avatar, skillNames, hostAgentId, hostAutoReplyEnabled, privateMessagesEnabled) =>
          void handleConfirmCreateRoom(ids, name, avatar, skillNames, hostAgentId, hostAutoReplyEnabled, privateMessagesEnabled)}
      />
    </div>
  );
});

export const ContactsSidebarPanelContent = memo(function ContactsSidebarPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const location = useLocation();
  const setActiveItem = useSidebarStore((s) => s.set_active_panel_item);
  const clearChatNotificationsForTarget = useSidebarStore(
    (s) => s.clear_chat_notifications_for_target,
  );
  const agentRuntimeStatuses = useAgentStore((s) => s.agent_runtime_statuses);
  const { agents, is_loading: isLoading } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const activeAgentId = location.pathname === AppRouteBuilders.contacts()
    ? new URLSearchParams(location.search).get("agent")
    : null;

  const filteredAgents = useMemo(() => {
    const normalizedQuery = normalize_query(query);
    if (!normalizedQuery) {
      return agents;
    }
    return agents.filter((agent) => agent.name.toLowerCase().includes(normalizedQuery));
  }, [agents, query]);

  const navigateToContacts = useCallback(() => {
    setActiveItem(null);
    if (location.pathname !== AppRouteBuilders.contacts() || location.search) {
      navigate(AppRouteBuilders.contacts());
    }
  }, [location.pathname, location.search, navigate, setActiveItem]);

  const navigateToAgentDetail = useCallback((agentId: string) => {
    setActiveItem(agentId);
    navigate(AppRouteBuilders.contact_agent(agentId));
  }, [navigate, setActiveItem]);

  const navigateToAgentDm = useCallback(async (agentId: string) => {
    const target = await resolve_direct_room_navigation_target(agentId);
    clearChatNotificationsForTarget(build_chat_notification_target_key({
      conversation_id: target.context.conversation.id,
      room_id: target.context.room.id,
    }));
    setActiveItem(target.context.room.id);
    navigate(target.route);
  }, [clearChatNotificationsForTarget, navigate, setActiveItem]);

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.contacts_list}>
      <SidebarSearchField
        action={(
          <button
            className="flex h-9 w-9 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_76%,transparent)] bg-[color:color-mix(in_srgb,var(--surface-elevated-background)_70%,transparent)] text-(--icon-muted) transition-[background,color,transform] duration-(--motion-duration-fast) hover:-translate-y-[1px] hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)"
            onClick={navigateToContacts}
            title={t("sidebar.manage_contacts")}
            type="button"
          >
            <UserPlus className="h-4 w-4" />
          </button>
        )}
        on_change={setQuery}
        placeholder={t("sidebar.search_contacts")}
        value={query}
      />

      {isLoading ? (
        <SidebarListLoadingRows />
      ) : (
        <div className="flex min-h-0 flex-1 flex-col gap-1 px-2 pb-2">
          {filteredAgents.length > 0 ? (
            filteredAgents.map((agent) => {
              const runningTaskCount = agentRuntimeStatuses[agent.id]?.running_task_count ?? 0;
              return (
                <ContactRow
                  agent={agent}
                  is_active={activeAgentId === agent.id}
                  is_working={runningTaskCount > 0}
                  key={agent.id}
                  on_chat={() => void navigateToAgentDm(agent.id)}
                  on_open_directory={() => navigateToAgentDetail(agent.id)}
                  running_task_count={runningTaskCount}
                />
              );
            })
          ) : (
            <SidebarEmptyGuide
              action_label={t("sidebar.manage_contacts")}
              description={t("sidebar.contacts_empty_description")}
              icon={Users2}
              on_action={navigateToContacts}
              title={query ? t("sidebar.no_matching_contacts") : t("sidebar.no_contacts")}
            />
          )}
        </div>
      )}
    </div>
  );
});
