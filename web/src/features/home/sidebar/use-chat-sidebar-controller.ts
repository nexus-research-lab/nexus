import { useCallback, useEffect, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { getActiveChatTargetFromPath } from "@/features/home/notifications/chat-notification-target";
import { createRoom, deleteRoom } from "@/lib/api/room-api";
import { useAgentStore } from "@/store/agent";
import { useSidebarStore } from "@/store/sidebar";

import {
  buildConversationItems,
  isMainAgentDmRoom,
  normalizeSidebarQuery,
  type SidebarConversationItem,
} from "./sidebar-conversation-model";
import { useSidebarDirectory } from "./sidebar-directory";
import { projectSidebarUnreadItems } from "./sidebar-unread-model";

interface DeleteTarget {
  id: string;
  name: string;
}

interface ChatSidebarControllerOptions {
  untitledRoomLabel: string;
}

export function useChatSidebarController({
  untitledRoomLabel,
}: ChatSidebarControllerOptions) {
  const location = useLocation();
  const navigate = useNavigate();
  const activeItemId = useSidebarStore((state) => state.active_panel_item_id);
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const chatUnreadCounts = useSidebarStore((state) => state.chat_unread_counts);
  const chatUnreadTargets = useSidebarStore((state) => state.chat_unread_targets);
  const chatUnreadTimestamps = useSidebarStore((state) => state.chat_unread_timestamps);
  const clearTargetNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_target,
  );
  const clearRoomNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_room,
  );
  const setNexusRoomId = useSidebarStore((state) => state.set_nexus_room_id);
  const agentRuntimeStatuses = useAgentStore((state) => state.agent_runtime_statuses);
  const {
    agents,
    conversations,
    isLoading,
    refreshDirectory,
    rooms,
  } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const nexusDmRoom = useMemo(
    () => rooms.find((room) => isMainAgentDmRoom(room)) ?? null,
    [rooms],
  );
  const activeTarget = useMemo(
    () => getActiveChatTargetFromPath(location.pathname),
    [location.pathname],
  );
  const conversationItems = useMemo(() => buildConversationItems({
    agents,
    agentRuntimeStatuses,
    conversations,
    rooms,
    untitledRoomLabel,
  }), [agents, agentRuntimeStatuses, conversations, rooms, untitledRoomLabel]);
  const items = useMemo(() => projectSidebarUnreadItems({
    activeTarget,
    chatUnreadCounts,
    chatUnreadTargets,
    chatUnreadTimestamps,
    items: conversationItems,
  }), [activeTarget, chatUnreadCounts, chatUnreadTargets, chatUnreadTimestamps, conversationItems]);
  const filteredItems = useMemo(
    () => filterConversationItems(items, query),
    [items, query],
  );

  useEffect(() => {
    setNexusRoomId(nexusDmRoom?.id ?? null);
  }, [nexusDmRoom, setNexusRoomId]);

  const openConversation = useCallback((item: SidebarConversationItem) => {
    const routeRoomId = item.routeRoomId ?? item.roomId;
    if (!routeRoomId) {
      return;
    }
    if (item.roomId) {
      clearRoomNotifications(item.roomId);
    }
    clearTargetNotifications(item.unreadTargetKey || item.notificationKey);
    setActiveItem(item.id);

    const conversationId = item.unreadConversationId || item.conversationId;
    const route = conversationId
      ? AppRouteBuilders.roomConversation(routeRoomId, conversationId)
      : AppRouteBuilders.room(routeRoomId);
    navigate(route);
  }, [clearRoomNotifications, clearTargetNotifications, navigate, setActiveItem]);

  const openContacts = useCallback(() => {
    navigate(AppRouteBuilders.contacts());
  }, [navigate]);

  const submitCreate = useCallback(async (submission: RoomDialogSubmission) => {
    setIsCreating(true);
    try {
      const context = await createRoom({
        agent_ids: submission.agentIds,
        avatar: submission.avatar,
        host_agent_id: submission.hostAgentId,
        host_auto_reply_enabled: submission.hostAutoReplyEnabled,
        name: submission.name,
        private_messages_enabled: submission.privateMessagesEnabled,
        skill_names: submission.skillNames,
      });
      setIsCreateOpen(false);
      refreshDirectory();
      navigate(AppRouteBuilders.room(context.room.id));
    } finally {
      setIsCreating(false);
    }
  }, [navigate, refreshDirectory]);

  const confirmDelete = useCallback(() => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    void deleteRoom(target.id)
      .then(() => {
        if (activeItemId === target.id) {
          setActiveItem(null);
        }
        refreshDirectory();
      })
      .catch((error) => {
        console.error("[Sidebar] 删除 Room 失败", error);
        refreshDirectory();
      });
  }, [activeItemId, deleteTarget, refreshDirectory, setActiveItem]);

  const requestDelete = useCallback((item: SidebarConversationItem) => {
    if (!item.canDelete || !item.roomId) {
      return;
    }
    setDeleteTarget({ id: item.roomId, name: item.title });
  }, []);

  const isItemActive = useCallback((item: SidebarConversationItem) => (
    activeItemId === item.id || Boolean(item.roomId && activeItemId === item.roomId)
  ), [activeItemId]);

  return {
    create: {
      cancel: () => setIsCreateOpen(false),
      isCreating,
      isOpen: isCreateOpen,
      open: () => setIsCreateOpen(true),
      submit: submitCreate,
    },
    deletion: {
      cancel: () => setDeleteTarget(null),
      confirm: confirmDelete,
      request: requestDelete,
      target: deleteTarget,
    },
    directory: {
      agents,
      hasAgents: agents.length > 0,
    },
    list: {
      isItemActive,
      isLoading,
      items: filteredItems,
      openConversation,
      query,
      setQuery,
    },
    navigation: {
      openContacts,
    },
  };
}

function filterConversationItems(
  items: SidebarConversationItem[],
  query: string,
): SidebarConversationItem[] {
  const normalizedQuery = normalizeSidebarQuery(query);
  if (!normalizedQuery) {
    return items;
  }
  return items.filter((item) => {
    const memberNames = item.members.map((member) => member.name).join(" ");
    return `${item.title} ${item.summary} ${memberNames}`
      .toLowerCase()
      .includes(normalizedQuery);
  });
}
