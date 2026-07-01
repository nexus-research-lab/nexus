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
  on_stop_message: (msg_id: string) => void;
  pending_permission_groups: Map<string, PendingPermission[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  send_permission_response: (payload: PermissionDecisionPayload) => boolean;
}

function get_thread_pending_permissions(
  round_id: string,
  agent_id: string,
  pending_permissions: PendingPermission[],
): PendingPermission[] {
  if (pending_permissions.length === 0) {
    return [];
  }

  return pending_permissions.filter((permission) => {
    if (permission.agent_id !== agent_id) {
      return false;
    }
    if (!permission.caused_by) {
      return false;
    }
    if (
      get_room_base_round_id(permission.caused_by, permission.agent_id) !==
      round_id
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
function derive_thread_panel_data(
  source: RoomThreadSource | null,
  active_thread: ThreadTarget | null,
): ThreadPanelData | null {
  if (!source || !active_thread) {
    return null;
  }

  const round_messages = source.message_groups.get(active_thread.round_id) ?? [];
  const messages = get_room_thread_messages(round_messages, active_thread.agent_id);
  const entry = get_room_agent_round_entry(
    round_messages,
    active_thread.agent_id,
    source.pending_slot_groups.get(active_thread.round_id) ?? [],
  );
  const is_loading = Boolean(entry && is_agent_round_active(entry.status));
  const agent_name = source.agent_name_map
    ? (source.agent_name_map[active_thread.agent_id] ?? active_thread.agent_id)
    : null;
  const agent_avatar = source.agent_avatar_map
    ? (source.agent_avatar_map[active_thread.agent_id] ?? null)
    : null;
  const pending_permissions = get_thread_pending_permissions(
    active_thread.round_id,
    active_thread.agent_id,
    source.pending_permission_groups.get(active_thread.round_id) ?? [],
  );

  return {
    messages,
    agent_name,
    agent_avatar,
    user_avatar: source.current_user_avatar,
    is_loading,
    pending_permissions,
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
  agent_avatar_map,
  agent_name_map,
  can_control_session,
  conversation_id,
  current_user_avatar,
  message_groups,
  observer_read_only_reason,
  on_open_workspace_file,
  on_stop_message,
  pending_permission_groups,
  pending_slot_groups,
  send_permission_response,
}: UseRoomThreadSourceOptions) {
  const { close_thread } = useGroupThread();
  const set_source = useRoomThreadLiveStore((state) => state.set_source);
  const clear_source = useRoomThreadLiveStore((state) => state.clear_source);

  const callbacks_ref = useRef({
    on_open_workspace_file,
    on_stop_message,
    send_permission_response,
  });
  useEffect(() => {
    callbacks_ref.current = {
      on_open_workspace_file,
      on_stop_message,
      send_permission_response,
    };
  }, [on_open_workspace_file, on_stop_message, send_permission_response]);

  const handle_permission_response = useCallback(
    (payload: PermissionDecisionPayload) =>
      callbacks_ref.current.send_permission_response(payload),
    [],
  );
  const handle_stop_message = useCallback((msg_id: string) => {
    callbacks_ref.current.on_stop_message(msg_id);
  }, []);
  const can_open_workspace_file = Boolean(on_open_workspace_file);
  const handle_open_workspace_file = useCallback((path: string) => {
    callbacks_ref.current.on_open_workspace_file?.(path);
  }, []);

  // 会话切换时收起 Thread。
  useEffect(() => {
    close_thread();
  }, [conversation_id, close_thread]);

  const source = useMemo<RoomThreadSource>(
    () => ({
      conversation_id,
      message_groups,
      pending_permission_groups,
      pending_slot_groups,
      agent_name_map,
      agent_avatar_map,
      current_user_avatar,
      can_control_session,
      observer_read_only_reason,
      on_permission_response: handle_permission_response,
      on_stop_message: handle_stop_message,
      on_open_workspace_file: can_open_workspace_file
        ? handle_open_workspace_file
        : undefined,
    }),
    [
      agent_avatar_map,
      agent_name_map,
      can_control_session,
      can_open_workspace_file,
      conversation_id,
      current_user_avatar,
      handle_open_workspace_file,
      handle_permission_response,
      handle_stop_message,
      message_groups,
      observer_read_only_reason,
      pending_permission_groups,
      pending_slot_groups,
    ],
  );

  // 入参（均已 memo / 稳定回调）不变时 source 引用恒定 → 仅真实更新才发布。
  useEffect(() => {
    set_source(source);
  }, [source, set_source]);

  // 卸载时清空，避免离开房间后残留陈旧切片。
  useEffect(() => {
    return () => {
      clear_source();
    };
  }, [clear_source]);
}

/**
 * 消费者侧：Thread 面板调用，读 active_thread + store source 派生展示数据。
 */
export function useRoomThreadPanel(): ThreadPanelData | null {
  const { active_thread } = useGroupThread();
  const source = useRoomThreadLiveStore((state) => state.source);
  return useMemo(
    () => derive_thread_panel_data(source, active_thread),
    [source, active_thread],
  );
}
