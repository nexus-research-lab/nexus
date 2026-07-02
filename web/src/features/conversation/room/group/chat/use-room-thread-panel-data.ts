"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import type { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import {
  get_room_agent_round_entry,
  get_room_base_round_id,
  get_room_thread_messages,
  is_agent_round_active,
} from "@/features/conversation/shared/utils";
import {
  useRoomThreadLiveStore,
  type RoomThreadSource,
} from "@/store/room-thread-live";
import { useGroupThread } from "../thread/group-thread-state";
import type {
  ThreadPanelData,
  ThreadTarget,
} from "../thread/group-thread-state";

interface UseRoomThreadSourceOptions {
  agent_avatar_map?: Record<string, string | null>;
  agent_name_map?: Record<string, string>;
  can_control_session: boolean;
  conversation_id: string | null;
  current_user_avatar?: string | null;
  message_groups: Map<string, Message[]>;
  observer_read_only_reason: string;
  on_open_workspace_file?: (path: string) => void;
  on_stop_message: (msgId: string) => void;
  pending_permission_groups: Map<string, PendingPermission[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  send_permission_response: (payload: PermissionDecisionPayload) => boolean;
}

function getThreadPendingPermissions(
  roundId: string,
  agentId: string,
  pendingPermissions: PendingPermission[],
): PendingPermission[] {
  if (pendingPermissions.length === 0) {
    return [];
  }

  return pendingPermissions.filter((permission) => {
    if (permission.agent_id !== agentId) {
      return false;
    }
    if (!permission.caused_by) {
      return false;
    }
    if (
      get_room_base_round_id(permission.caused_by, permission.agent_id) !==
      roundId
    ) {
      return false;
    }
    // Room 的权限请求在很多场景下绑定的是占位槽位 msg_id，
    // 不是 assistant 真正的 message_id。Thread 已经按 round_id + agent_id 收口，
    // 这里不能再按 message_id 二次过滤，否则问答/权限会被错误吞掉。
    return true;
  });
}

/**
 * 由 source（GroupChatPanel 发布的会话切片）+ active_thread 派生出 Thread 面板数据。
 * 纯函数，无副作用——在消费者 render 内调用，不写回渲染周期。
 */
function deriveThreadPanelData(
  source: RoomThreadSource | null,
  activeThread: ThreadTarget | null,
): ThreadPanelData | null {
  if (!source || !activeThread) {
    return null;
  }

  const roundMessages = source.message_groups.get(activeThread.round_id) ?? [];
  const messages = get_room_thread_messages(roundMessages, activeThread.agent_id);
  const entry = get_room_agent_round_entry(
    roundMessages,
    activeThread.agent_id,
    source.pending_slot_groups.get(activeThread.round_id) ?? [],
  );
  const isLoading = Boolean(entry && is_agent_round_active(entry.status));
  const agentName = source.agent_name_map
    ? (source.agent_name_map[activeThread.agent_id] ?? activeThread.agent_id)
    : null;
  const agentAvatar = source.agent_avatar_map
    ? (source.agent_avatar_map[activeThread.agent_id] ?? null)
    : null;
  const pendingPermissions = getThreadPendingPermissions(
    activeThread.round_id,
    activeThread.agent_id,
    source.pending_permission_groups.get(activeThread.round_id) ?? [],
  );

  return {
    messages,
    agent_name: agentName,
    agent_avatar: agentAvatar,
    user_avatar: source.current_user_avatar,
    is_loading: isLoading,
    pending_permissions: pendingPermissions,
    on_permission_response: source.on_permission_response,
    can_respond_to_permissions: source.can_control_session,
    permission_read_only_reason: source.observer_read_only_reason,
    on_stop_message: source.can_control_session ? source.on_stop_message : undefined,
    on_open_workspace_file: source.on_open_workspace_file,
  };
}

/**
 * 生产者侧：把会话切片发布到 room-thread-live store。
 * 不订阅 store → 写入不会重渲染自己 → 无反馈环。
 */
export function useRoomThreadSource({
  agent_avatar_map: agentAvatarMap,
  agent_name_map: agentNameMap,
  can_control_session: canControlSession,
  conversation_id: conversationId,
  current_user_avatar: currentUserAvatar,
  message_groups: messageGroups,
  observer_read_only_reason: observerReadOnlyReason,
  on_open_workspace_file: onOpenWorkspaceFile,
  on_stop_message: onStopMessage,
  pending_permission_groups: pendingPermissionGroups,
  pending_slot_groups: pendingSlotGroups,
  send_permission_response: sendPermissionResponse,
}: UseRoomThreadSourceOptions) {
  const { close_thread: closeThread } = useGroupThread();
  const setSource = useRoomThreadLiveStore((state) => state.set_source);
  const clearSource = useRoomThreadLiveStore((state) => state.clear_source);

  const callbacksRef = useRef({
    on_open_workspace_file: onOpenWorkspaceFile,
    on_stop_message: onStopMessage,
    send_permission_response: sendPermissionResponse,
  });
  useEffect(() => {
    callbacksRef.current = {
      on_open_workspace_file: onOpenWorkspaceFile,
      on_stop_message: onStopMessage,
      send_permission_response: sendPermissionResponse,
    };
  }, [onOpenWorkspaceFile, onStopMessage, sendPermissionResponse]);

  const handlePermissionResponse = useCallback(
    (payload: PermissionDecisionPayload) =>
      callbacksRef.current.send_permission_response(payload),
    [],
  );
  const handleStopMessage = useCallback((msgId: string) => {
    callbacksRef.current.on_stop_message(msgId);
  }, []);
  const canOpenWorkspaceFile = Boolean(onOpenWorkspaceFile);
  const handleOpenWorkspaceFile = useCallback((path: string) => {
    callbacksRef.current.on_open_workspace_file?.(path);
  }, []);

  // 会话切换时收起 Thread。
  useEffect(() => {
    closeThread();
  }, [conversationId, closeThread]);

  const source = useMemo<RoomThreadSource>(
    () => ({
      conversation_id: conversationId,
      message_groups: messageGroups,
      pending_permission_groups: pendingPermissionGroups,
      pending_slot_groups: pendingSlotGroups,
      agent_name_map: agentNameMap,
      agent_avatar_map: agentAvatarMap,
      current_user_avatar: currentUserAvatar,
      can_control_session: canControlSession,
      observer_read_only_reason: observerReadOnlyReason,
      on_permission_response: handlePermissionResponse,
      on_stop_message: handleStopMessage,
      on_open_workspace_file: canOpenWorkspaceFile
        ? handleOpenWorkspaceFile
        : undefined,
    }),
    [
      agentAvatarMap,
      agentNameMap,
      canControlSession,
      canOpenWorkspaceFile,
      conversationId,
      currentUserAvatar,
      handleOpenWorkspaceFile,
      handlePermissionResponse,
      handleStopMessage,
      messageGroups,
      observerReadOnlyReason,
      pendingPermissionGroups,
      pendingSlotGroups,
    ],
  );

  // 入参（均已 memo / 稳定回调）不变时 source 引用恒定 → 仅真实更新才发布。
  useEffect(() => {
    setSource(source);
  }, [source, setSource]);

  // 卸载时清空，避免离开房间后残留陈旧切片。
  useEffect(() => {
    return () => {
      clearSource();
    };
  }, [clearSource]);
}

/**
 * 消费者侧：Thread 面板调用，读 active_thread + store source 派生展示数据。
 */
export function useRoomThreadPanel(): ThreadPanelData | null {
  const { active_thread: activeThread } = useGroupThread();
  const source = useRoomThreadLiveStore((state) => state.source);
  return useMemo(
    () => deriveThreadPanelData(source, activeThread),
    [source, activeThread],
  );
}
