/**
 * INPUT: Home 聊天目录、活动路由、Room/DM 未读状态与用户列表命令。
 * OUTPUT: 侧栏筛选/创建/删除/导航控制器；Room 导航保留锚点给 Feed 消费。
 * POS: Home 聊天侧栏有状态装配入口。
 */
import { useCallback, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { getDefaultAgentId, isMainAgent, USER_PREFERENCES_CHANGED_EVENT } from "@/config/runtime-options";
import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import type { RoomDialogSubmission } from "@/features/conversation/room/members/create-room-dialog";
import { getActiveChatTargetFromPath } from "@/features/home/notifications/chat-notification-target";
import { useTeamRooms } from "@/features/team/use-team-rooms";
import { createTeamRoom } from "@/lib/api/conversation/team-api";
import { createRoom, deleteRoom } from "@/lib/api/conversation/room-command-api";
import { projectMutationFailure } from "@/lib/error-message";
import {
  captureAuthOwnerScopeGeneration,
  isAuthOwnerScopeGenerationCurrent,
} from "@/shared/auth/auth-owner-generation";
import { useI18n } from "@/shared/i18n/i18n-context";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { useSidebarStore } from "@/store/sidebar";

import { useRoomActivity } from "../room-activity-resource";
import {
  buildConversationItems,
  buildTeamConversationItem,
  sortConversationItems,
  type SidebarConversationItem,
} from "./sidebar-conversation-model";
import { useSidebarDirectory } from "./sidebar-directory";
import { projectSidebarUnreadItems } from "./sidebar-unread-model";
import {
  getRoomDeletionCommand,
  projectRoomDeletionFailure,
  type RoomDeletionFailure,
} from "./room-deletion-recovery";

interface DeleteTarget {
  agentId?: string;
  id: string;
  name: string;
}

interface ChatSidebarControllerOptions {
  untitledRoomLabel: string;
}

export function useChatSidebarController({
  untitledRoomLabel,
}: ChatSidebarControllerOptions) {
  const { locale, t } = useI18n();
  const mainAgentId = useSyncExternalStore(subscribeRuntimeIdentity, getDefaultAgentId, getDefaultAgentId);
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
  const onlineRooms = useTeamRooms();
  const {
    agents,
    conversations,
    hasError,
    hasLoaded,
    isLoading,
    reconcileRoomTarget,
    refreshDirectory,
    rooms,
  } = useSidebarDirectory();
  const [query, setQuery] = useState("");
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);
  const [deleteAction, setDeleteAction] = useState<"check" | "delete" | null>(null);
  const [deleteFailure, setDeleteFailure] = useState<RoomDeletionFailure | null>(null);
  const deletionRunningRef = useRef(false);
  const unresolvedDeletionsRef = useRef(new Map<string, {
    failure: RoomDeletionFailure;
    ownerGeneration: number;
  }>());
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [isOnlineCreateOpen, setIsOnlineCreateOpen] = useState(false);
  const [isOnlineCreating, setIsOnlineCreating] = useState(false);
  const [onlineCreateError, setOnlineCreateError] = useState(false);
  const onlineCreateCommandRef = useRef<{ id: string; name: string } | null>(null);
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
    mainAgentId,
  }), [
    agents,
    conversations,
    locale,
    mainAgentId,
    roomActivity,
    rooms,
    untitledRoomLabel,
  ]);
  const items = useMemo(() => {
    const localItems = projectSidebarUnreadItems({
      activeTarget,
      chatUnreadAnchors,
      chatUnreadCounts,
      chatUnreadTargets,
      chatUnreadTimestamps,
      items: conversationItems,
    });
    if (onlineRooms.rooms.length === 0) {
      return localItems;
    }
    return sortConversationItems([
      ...localItems,
      ...onlineRooms.rooms.map((team) => buildTeamConversationItem({
        fallbackTitle: t("team.shared_room"),
        locale,
        summary: t("team.shared_room_summary"),
        team,
      })),
    ], locale);
  }, [
    activeTarget,
    chatUnreadAnchors,
    chatUnreadCounts,
    chatUnreadTargets,
    chatUnreadTimestamps,
    conversationItems,
    locale,
    onlineRooms.rooms,
    t,
  ]);
  const filteredItems = useMemo(
    () => filterConversationItems(items, query),
    [items, query],
  );

  const openConversation = useCallback((item: SidebarConversationItem) => {
    if (item.kind === "team") {
      setActiveItem(item.id);
      navigate(AppRouteBuilders.team(item.roomId));
      return;
    }
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

  const finishDeletion = useCallback((target: DeleteTarget) => {
    unresolvedDeletionsRef.current.delete(target.id);
    setDeleteTarget(null);
    setDeleteFailure(null);
    discardRoomChatState(target.id);
    if (activeItemId === target.id) {
      setActiveItem(null);
    }
  }, [activeItemId, discardRoomChatState, setActiveItem]);

  const reconcileDeletion = useCallback(async (
    target: DeleteTarget,
    failure: RoomDeletionFailure,
    ownerGeneration: number,
  ) => {
    try {
      const targetStillExists = await reconcileRoomTarget(target.id);
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      if (failure.kind === "committed_cleanup_incomplete") {
        const next = {
          ...failure,
          directoryCheck: targetStillExists ? "target_present" : "target_absent",
        } as const;
        unresolvedDeletionsRef.current.set(target.id, { failure: next, ownerGeneration });
        setDeleteFailure(next);
        return;
      }
      if (targetStillExists) {
        const next = {
          directoryCheck: "target_present",
          kind: failure.kind === "resource_absent"
            ? "resource_absent"
            : "outcome_unknown",
        } as const;
        unresolvedDeletionsRef.current.set(target.id, { failure: next, ownerGeneration });
        setDeleteFailure(next);
        return;
      }
      finishDeletion(target);
    } catch {
      if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
        return;
      }
      const next = { ...failure, directoryCheck: "failed" } as const;
      unresolvedDeletionsRef.current.set(target.id, { failure: next, ownerGeneration });
      setDeleteFailure(next);
    }
  }, [finishDeletion, reconcileRoomTarget]);

  const confirmDelete = useCallback(async () => {
    if (!deleteTarget || deletionRunningRef.current || (deleteTarget.agentId && isMainAgent(deleteTarget.agentId))) {
      return;
    }
    deletionRunningRef.current = true;
    const ownerGeneration = captureAuthOwnerScopeGeneration();
    const target = deleteTarget;
    try {
      const command = getRoomDeletionCommand(deleteFailure);
      if (command === "dismiss") {
        finishDeletion(target);
        return;
      }
      if (deleteFailure && command === "reconcile") {
        setDeleteAction("check");
        await reconcileDeletion(target, deleteFailure, ownerGeneration);
        return;
      }

      setDeleteAction("delete");
      setDeleteFailure(null);
      try {
        await deleteRoom(target.id);
        if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          return;
        }
        finishDeletion(target);
        refreshDirectory();
      } catch (error) {
        if (!isAuthOwnerScopeGenerationCurrent(ownerGeneration)) {
          return;
        }
        const failure = projectRoomDeletionFailure(projectMutationFailure(
          error,
          "Room 删除结果暂时无法确认",
        ));
        if (failure.kind === "not_applied") {
          unresolvedDeletionsRef.current.delete(target.id);
        } else {
          unresolvedDeletionsRef.current.set(target.id, { failure, ownerGeneration });
        }
        setDeleteFailure(failure);
        if (failure.kind !== "not_applied") {
          setDeleteAction("check");
          await reconcileDeletion(target, failure, ownerGeneration);
        }
      }
    } finally {
      deletionRunningRef.current = false;
      setDeleteAction(null);
    }
  }, [
    deleteFailure,
    deleteTarget,
    finishDeletion,
    reconcileDeletion,
    refreshDirectory,
  ]);

  const requestDelete = useCallback((item: SidebarConversationItem) => {
    if (deletionRunningRef.current || !item.canDelete || !item.roomId || (item.kind === "dm" && isMainAgent(item.agentId))) {
      return;
    }
    const unresolved = unresolvedDeletionsRef.current.get(item.roomId);
    setDeleteFailure(
      unresolved?.ownerGeneration === captureAuthOwnerScopeGeneration()
        ? unresolved.failure
        : null,
    );
    setDeleteTarget({ id: item.roomId, name: item.title, agentId: item.kind === "dm" ? item.agentId : undefined });
  }, []);

  const cancelDelete = useCallback(() => {
    if (deletionRunningRef.current) {
      return;
    }
    // 关闭只隐藏确认框；结果未知的原删除仍按 exact Room 保留，
    // 再次打开时继续核对，不能伪装成一项全新的安全删除。
    setDeleteFailure(null);
    setDeleteTarget(null);
  }, []);

  const isItemActive = useCallback((item: SidebarConversationItem) => {
    if (item.kind === "team") {
      const selectedRoomId = new URLSearchParams(location.search).get("room_id")
        ?? onlineRooms.rooms[0]?.room.id;
      return location.pathname === AppRouteBuilders.team() && selectedRoomId === item.roomId;
    }
    return activeItemId === item.id || Boolean(item.roomId && activeItemId === item.roomId);
  }, [activeItemId, location.pathname, location.search, onlineRooms.rooms]);

  const submitOnlineCreate = useCallback(async (name: string) => {
    const normalized = name.trim();
    if (!normalized || isOnlineCreating) {
      return;
    }
    const command = onlineCreateCommandRef.current?.name === normalized
      ? onlineCreateCommandRef.current
      : { id: crypto.randomUUID(), name: normalized };
    onlineCreateCommandRef.current = command;
    setIsOnlineCreating(true);
    setOnlineCreateError(false);
    try {
      const created = await createTeamRoom(normalized, command.id);
      onlineCreateCommandRef.current = null;
      setIsOnlineCreateOpen(false);
      onlineRooms.refresh();
      navigate(AppRouteBuilders.team(created.room.id));
    } catch {
      setOnlineCreateError(true);
    } finally {
      setIsOnlineCreating(false);
    }
  }, [isOnlineCreating, navigate, onlineRooms]);

  return {
    create: {
      cancel: () => setIsCreateOpen(false),
      isCreating,
      isOpen: isCreateOpen,
      open: () => setIsCreateOpen(true),
      submit: submitCreate,
    },
    onlineCreate: {
      cancel: () => {
        if (!isOnlineCreating) {
          setIsOnlineCreateOpen(false);
          setOnlineCreateError(false);
        }
      },
      error: onlineCreateError,
      isAvailable: onlineRooms.isAvailable,
      isCreating: isOnlineCreating,
      isOpen: isOnlineCreateOpen,
      open: () => {
        setOnlineCreateError(false);
        setIsOnlineCreateOpen(true);
      },
      submit: submitOnlineCreate,
    },
    deletion: {
      action: deleteAction,
      cancel: cancelDelete,
      confirm: confirmDelete,
      failure: deleteFailure,
      request: requestDelete,
      target: deleteTarget,
    },
    directory: {
      agents,
    },
    list: {
      hasError,
      hasLoaded,
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
  const search = createUiSearchMatcher(query);
  return items.filter((item) => {
    return search.matches([
      item.title,
      item.summary,
      ...item.members.map((member) => member.name),
    ]);
  });
}

function subscribeRuntimeIdentity(listener: () => void): () => void {
  window.addEventListener(USER_PREFERENCES_CHANGED_EVENT, listener);
  return () => window.removeEventListener(USER_PREFERENCES_CHANGED_EVENT, listener);
}
