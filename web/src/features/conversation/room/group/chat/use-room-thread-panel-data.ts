"use client";

import { useCallback, useEffect, useMemo, useRef } from "react";

import type { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import {
  getRoomAgentRoundEntry,
  getRoomThreadMessages,
  isAgentRoundActive,
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
  agentAvatarMap?: Record<string, string | null>;
  agentNameMap?: Record<string, string>;
  conversationId: string | null;
  currentUserAvatar?: string | null;
  messageGroups: Map<string, Message[]>;
  onOpenWorkspaceFile?: (path: string, workspaceAgentId?: string | null) => void;
  onStopMessage: (msgId: string) => void;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  sendPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
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
    // 权限事件带显式 root round_id，直接精确匹配。
    return permission.round_id === roundId;
  });
}

/**
 * 由 source（GroupChatPanel 发布的会话切片）+ activeThread 派生出 Thread 面板数据。
 * 纯函数，无副作用——在消费者 render 内调用，不写回渲染周期。
 */
function deriveThreadPanelData(
  source: RoomThreadSource | null,
  activeThread: ThreadTarget | null,
): ThreadPanelData | null {
  if (!source || !activeThread) {
    return null;
  }

  const roundMessages = source.message_groups.get(activeThread.roundId) ?? [];
  const messages = getRoomThreadMessages(roundMessages, activeThread.agentId);
  const entry = getRoomAgentRoundEntry(
    roundMessages,
    activeThread.agentId,
    source.pending_slot_groups.get(activeThread.roundId) ?? [],
  );
  const isLoading = Boolean(entry && isAgentRoundActive(entry.status));
  const agentName = source.agent_name_map
    ? (source.agent_name_map[activeThread.agentId] ?? activeThread.agentId)
    : null;
  const agentAvatar = source.agent_avatar_map
    ? (source.agent_avatar_map[activeThread.agentId] ?? null)
    : null;
  const pendingPermissions = getThreadPendingPermissions(
    activeThread.roundId,
    activeThread.agentId,
    source.pending_permission_groups.get(activeThread.roundId) ?? [],
  );

  return {
    messages,
    agentName,
    agentAvatar,
    userAvatar: source.current_user_avatar,
    isLoading,
    pendingPermissions,
    onPermissionResponse: source.on_permission_response,
    onStopMessage: source.on_stop_message,
    onOpenWorkspaceFile: source.on_open_workspace_file,
  };
}

/**
 * 生产者侧：把会话切片发布到 room-thread-live store。
 * 不订阅 store → 写入不会重渲染自己 → 无反馈环。
 */
export function useRoomThreadSource({
  agentAvatarMap,
  agentNameMap,
  conversationId,
  currentUserAvatar,
  messageGroups,
  onOpenWorkspaceFile,
  onStopMessage,
  pendingPermissionGroups,
  pendingSlotGroups,
  sendPermissionResponse,
}: UseRoomThreadSourceOptions) {
  const { closeThread } = useGroupThread();
  const setSource = useRoomThreadLiveStore((state) => state.set_source);
  const clearSource = useRoomThreadLiveStore((state) => state.clear_source);

  const callbacksRef = useRef({
    onOpenWorkspaceFile,
    onStopMessage,
    sendPermissionResponse,
  });
  useEffect(() => {
    callbacksRef.current = {
      onOpenWorkspaceFile,
      onStopMessage,
      sendPermissionResponse,
    };
  }, [onOpenWorkspaceFile, onStopMessage, sendPermissionResponse]);

  const handlePermissionResponse = useCallback(
    (payload: PermissionDecisionPayload) =>
      callbacksRef.current.sendPermissionResponse(payload),
    [],
  );
  const handleStopMessage = useCallback((msgId: string) => {
    callbacksRef.current.onStopMessage(msgId);
  }, []);
  const canOpenWorkspaceFile = Boolean(onOpenWorkspaceFile);
  const handleOpenWorkspaceFile = useCallback((path: string, workspaceAgentId?: string | null) => {
    callbacksRef.current.onOpenWorkspaceFile?.(path, workspaceAgentId);
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
      on_permission_response: handlePermissionResponse,
      on_stop_message: handleStopMessage,
      on_open_workspace_file: canOpenWorkspaceFile
        ? handleOpenWorkspaceFile
        : undefined,
    }),
    [
      agentAvatarMap,
      agentNameMap,
      canOpenWorkspaceFile,
      conversationId,
      currentUserAvatar,
      handleOpenWorkspaceFile,
      handlePermissionResponse,
      handleStopMessage,
      messageGroups,
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
 * 消费者侧：Thread 面板调用，读 activeThread + store source 派生展示数据。
 */
export function useRoomThreadPanel(): ThreadPanelData | null {
  const { activeThread } = useGroupThread();
  const source = useRoomThreadLiveStore((state) => state.source);
  return useMemo(
    () => deriveThreadPanelData(source, activeThread),
    [source, activeThread],
  );
}
