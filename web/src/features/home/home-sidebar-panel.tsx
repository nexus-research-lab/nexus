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
import { createRoom, deleteRoom } from "@/lib/api/room-api";
import { resolveDirectRoomNavigationTarget } from "@/lib/conversation/direct-room-navigation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { SidebarEmptyGuide } from "@/shared/ui/sidebar/sidebar-empty-guide";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";
import {
  buildChatNotificationTargetKey,
  getActiveChatTargetFromPath,
} from "./chat-notification-target";
import {
  buildConversationItems,
  buildSidebarItemNotificationKey,
  getSidebarItemUnreadState,
  isActiveSidebarChatItem,
  isMainAgentDmRoom,
  normalizeQuery,
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
  roomType: "room" | "dm";
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
  const { agents, conversations, isLoading, refreshDirectory, rooms } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [isCreateRoomOpen, setIsCreateRoomOpen] = useState(false);
  const [isCreatingRoom, setIsCreatingRoom] = useState(false);
  const untitledRoomLabel = t("home.untitled_room");
  const hasAgents = agents.length > 0;

  const nexusDmRoom = useMemo(
    () => rooms.find((room) => isMainAgentDmRoom(room)) ?? null,
    [rooms],
  );
  const activeChatTarget = useMemo(
    () => getActiveChatTargetFromPath(location.pathname),
    [location.pathname],
  );

  useEffect(() => {
    setNexusRoomId(nexusDmRoom?.id ?? null);
  }, [nexusDmRoom, setNexusRoomId]);

  const rawItems = useMemo(
    () => buildConversationItems({
      agents,
      agentRuntimeStatuses,
      conversations,
      formatRunningTasksSummary: (count) => t("sidebar.running_tasks_summary", { count }),
      rooms,
      untitledRoomLabel,
    }).map((item) => {
      const notificationKey = buildSidebarItemNotificationKey(item);
      const unreadState = getSidebarItemUnreadState({
        chatUnreadCounts,
        chatUnreadTargets,
        chatUnreadTimestamps,
        notificationKey,
        roomId: item.roomId,
        sessionKey: item.sessionKey,
      });
      return {
        ...item,
        notificationKey,
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
      const visibleUnreadState = isActiveSidebarChatItem(item, activeChatTarget)
        ? {
          unreadConversationId: null,
          unreadCount: 0,
          unreadTargetKey: null,
        }
        : {
          unreadConversationId: item.unreadConversationId ?? null,
          unreadCount: item.unreadCount ?? 0,
          unreadTargetKey: item.unreadTargetKey ?? null,
        };
      return {
        ...item,
        ...visibleUnreadState,
      };
    }),
    [activeChatTarget, rawItems],
  );

  const filteredItems = useMemo(() => {
    const normalizedQuery = normalizeQuery(query);
    if (!normalizedQuery) {
      return items;
    }
    return items.filter((item) => {
      const memberNames = item.members.map((member) => member.name).join(" ");
      return `${item.title} ${item.summary} ${memberNames}`.toLowerCase().includes(normalizedQuery);
    });
  }, [items, query]);

  const navigateToRoom = useCallback(async (item: SidebarConversationItem) => {
    const routeRoomId = item.routeRoomId ?? item.roomId;
    if (!routeRoomId) {
      return;
    }
    const targetConversationId = item.unreadConversationId || item.conversationId;
    if (item.roomId) {
      clearChatNotificationsForRoom(item.roomId);
    }
    clearChatNotificationsForTarget(item.unreadTargetKey || item.notificationKey);
    setActiveItem(item.id);
    if (targetConversationId) {
      navigate(AppRouteBuilders.roomConversation(routeRoomId, targetConversationId));
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
      const context = await createRoom({
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
    await deleteRoom(deletedRoomId);
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
        onChange={setQuery}
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
                isActive={activeItemId === item.id || (item.roomId ? activeItemId === item.roomId : false)}
                item={item}
                key={item.id}
                onClick={() => {
                  void navigateToRoom(item);
                }}
                onDelete={item.canDelete && item.roomId ? () => setDeleteTarget({
                  id: item.roomId ?? item.id,
                  name: item.title,
                  roomType: item.kind,
                }) : undefined}
              />
            ))
          ) : (
            <SidebarEmptyGuide
              actionLabel={emptyAction}
              description={emptyDescription}
              icon={MessageSquarePlus}
              onAction={handleEmptyAction}
              title={query ? t("sidebar.no_matching_conversations") : t("home.rooms_empty_title")}
            />
          )}
        </div>
      )}

      <ConfirmDialog
        confirmText={t("common.delete")}
        isOpen={deleteTarget !== null}
        message={t("home.delete_message", { name: deleteTarget?.name ?? "" })}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={handleConfirmDeleteRoom}
        title={t("home.delete_confirm")}
        variant="danger"
      />

      <CreateRoomDialog
        agents={agents.map((agent) => ({
          agent_id: agent.id,
          name: agent.name,
          avatar: agent.avatar,
        }))}
        isCreating={isCreatingRoom}
        isOpen={isCreateRoomOpen}
        onCancel={() => setIsCreateRoomOpen(false)}
        onConfirm={(ids, name, avatar, skillNames, hostAgentId, hostAutoReplyEnabled, privateMessagesEnabled) =>
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
  const { agents, isLoading } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const activeAgentId = location.pathname === AppRouteBuilders.contacts()
    ? new URLSearchParams(location.search).get("agent")
    : null;

  const filteredAgents = useMemo(() => {
    const normalizedQuery = normalizeQuery(query);
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
    navigate(AppRouteBuilders.contactAgent(agentId));
  }, [navigate, setActiveItem]);

  const navigateToAgentDm = useCallback(async (agentId: string) => {
    const target = await resolveDirectRoomNavigationTarget(agentId);
    clearChatNotificationsForTarget(buildChatNotificationTargetKey({
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
        onChange={setQuery}
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
                  isActive={activeAgentId === agent.id}
                  isWorking={runningTaskCount > 0}
                  key={agent.id}
                  onChat={() => void navigateToAgentDm(agent.id)}
                  onOpenDirectory={() => navigateToAgentDetail(agent.id)}
                  runningTaskCount={runningTaskCount}
                />
              );
            })
          ) : (
            <SidebarEmptyGuide
              actionLabel={t("sidebar.manage_contacts")}
              description={t("sidebar.contacts_empty_description")}
              icon={Users2}
              onAction={navigateToContacts}
              title={query ? t("sidebar.no_matching_contacts") : t("sidebar.no_contacts")}
            />
          )}
        </div>
      )}
    </div>
  );
});
