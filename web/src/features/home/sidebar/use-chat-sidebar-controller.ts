/**
 * INPUT: Home 聊天目录、活动路由、Room/DM 未读状态与用户列表命令。
 * OUTPUT: 侧栏筛选/创建/删除/导航控制器；Room 导航保留锚点给 Feed 消费。
 * POS: Home 聊天侧栏有状态装配入口。
 */
import { useCallback, useMemo, useState } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { getActiveChatTargetFromPath } from "@/features/home/notifications/chat-notification-target";
import { createRoom, deleteRoom } from "@/lib/api/conversation/room-command-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useSidebarStore } from "@/store/sidebar";

import { useRoomActivity } from "../room-activity-resource";
import {
  buildConversationItems,
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
  const { locale } = useI18n();
  const location = useLocation();
  const navigate = useNavigate();
  const activeItemId = useSidebarStore((state) => state.active_panel_item_id);
  const setActiveItem = useSidebarStore((state) => state.set_active_panel_item);
  const chatUnreadAnchors = useSidebarStore((state) => state.chat_unread_anchors);
  const chatUnreadCounts = useSidebarStore((state) => state.chat_unread_counts);
  const chatUnreadTargets = useSidebarStore((state) => state.chat_unread_targets);
  const chatUnreadTimestamps = useSidebarStore((state) => state.chat_unread_timestamps);
  const clearTargetNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_target,
  );
  const clearRoomNotifications = useSidebarStore(
    (state) => state.clear_chat_notifications_for_room,
  );
  const discardRoomChatState = useSidebarStore(
    (state) => state.discard_chat_state_for_room,
  );
  const roomActivity = useRoomActivity();
  const {
    agents,
    conversations,
    hasError,
    isLoading,
    refreshDirectory,
    rooms,
  } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const activeTarget = useMemo(
    () => getActiveChatTargetFromPath(location.pathname),
    [location.pathname],
  );
  const conversationItems = useMemo(() => buildConversationItems({
    agents,
    conversations,
    locale,
    rooms,
    untitledRoomLabel,
    roomActivity,
  }), [
    agents,
    conversations,
    locale,
    roomActivity,
    rooms,
    untitledRoomLabel,
  ]);
  const items = useMemo(() => projectSidebarUnreadItems({
    activeTarget,
    chatUnreadAnchors,
    chatUnreadCounts,
    chatUnreadTargets,
    chatUnreadTimestamps,
    items: conversationItems,
  }), [
    activeTarget,
    chatUnreadAnchors,
    chatUnreadCounts,
    chatUnreadTargets,
    chatUnreadTimestamps,
    conversationItems,
  ]);
  const filteredItems = useMemo(
    () => filterConversationItems(items, query),
    [items, query],
  );

  const openConversation = useCallback((item: SidebarConversationItem) => {
    const routeRoomId = item.routeRoomId ?? item.roomId;
    if (!routeRoomId) {
      return;
    }
    if (item.kind === "dm" && item.roomId) {
      clearRoomNotifications(item.roomId);
      clearTargetNotifications(item.unreadTargetKey || item.notificationKey);
    }
    setActiveItem(item.id);

    const route = item.unreadConversationId
      ? AppRouteBuilders.roomConversation(routeRoomId, item.unreadConversationId)
      : AppRouteBuilders.room(routeRoomId);
    navigate(route);
  }, [clearRoomNotifications, clearTargetNotifications, navigate, setActiveItem]);

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
        discardRoomChatState(target.id);
        if (activeItemId === target.id) {
          setActiveItem(null);
        }
        refreshDirectory();
      })
      .catch((error) => {
        console.error("[Sidebar] 删除 Room 失败", error);
        refreshDirectory();
      });
  }, [
    activeItemId,
    deleteTarget,
    discardRoomChatState,
    refreshDirectory,
    setActiveItem,
  ]);

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
    },
    list: {
      hasError,
      isItemActive,
      isLoading,
      items: filteredItems,
      openConversation,
      query,
      retry: refreshDirectory,
      setQuery,
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
