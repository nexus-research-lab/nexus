"use client";

import { useEffect, useMemo } from "react";

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
  useGroupThread,
  useSetGroupThreadPanelData,
} from "../thread/group-thread-state";

interface UseRoomThreadPanelDataOptions {
  agent_avatar_map?: Record<string, string | null>;
  agent_name_map?: Record<string, string>;
  can_control_session: boolean;
  conversation_id: string | null;
  current_user_avatar?: string | null;
  is_loading: boolean;
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

export function useRoomThreadPanelData({
  agent_avatar_map,
  agent_name_map,
  can_control_session,
  conversation_id,
  current_user_avatar,
  is_loading,
  message_groups,
  observer_read_only_reason,
  on_open_workspace_file,
  on_stop_message,
  pending_permission_groups,
  pending_slot_groups,
  send_permission_response,
}: UseRoomThreadPanelDataOptions) {
  const { active_thread, close_thread } = useGroupThread();
  const { set_thread_panel_data } = useSetGroupThreadPanelData();

  useEffect(() => {
    close_thread();
  }, [conversation_id, close_thread]);

  const thread_round_messages = useMemo(
    () =>
      active_thread ? (message_groups.get(active_thread.round_id) ?? []) : [],
    [active_thread, message_groups],
  );
  const thread_messages = useMemo(() => {
    if (!active_thread) {
      return [];
    }

    return get_room_thread_messages(
      thread_round_messages,
      active_thread.agent_id,
    );
  }, [active_thread, thread_round_messages]);
  const thread_entry = useMemo(
    () =>
      active_thread
        ? get_room_agent_round_entry(
            thread_round_messages,
            active_thread.agent_id,
            pending_slot_groups.get(active_thread.round_id) ?? [],
          )
        : null,
    [active_thread, pending_slot_groups, thread_round_messages],
  );
  const thread_is_loading = useMemo(
    () => Boolean(thread_entry && is_agent_round_active(thread_entry.status)),
    [thread_entry],
  );
  const thread_agent_name =
    active_thread && agent_name_map
      ? (agent_name_map[active_thread.agent_id] ?? active_thread.agent_id)
      : null;
  const thread_agent_avatar =
    active_thread && agent_avatar_map
      ? (agent_avatar_map[active_thread.agent_id] ?? null)
      : null;
  const thread_pending_permissions = useMemo(
    () =>
      active_thread
        ? get_thread_pending_permissions(
            active_thread.round_id,
            active_thread.agent_id,
            pending_permission_groups.get(active_thread.round_id) ?? [],
          )
        : [],
    [active_thread, pending_permission_groups],
  );
  const thread_panel_data = useMemo(() => {
    if (!active_thread) {
      return null;
    }

    return {
      messages: thread_messages,
      agent_name: thread_agent_name,
      agent_avatar: thread_agent_avatar,
      user_avatar: current_user_avatar,
      is_loading: thread_is_loading,
      pending_permissions: thread_pending_permissions,
      on_permission_response: send_permission_response,
      can_respond_to_permissions: can_control_session,
      permission_read_only_reason: observer_read_only_reason,
      on_stop_message: can_control_session ? on_stop_message : undefined,
      on_open_workspace_file,
    };
  }, [
    active_thread,
    can_control_session,
    current_user_avatar,
    observer_read_only_reason,
    on_open_workspace_file,
    on_stop_message,
    send_permission_response,
    thread_agent_avatar,
    thread_agent_name,
    thread_is_loading,
    thread_messages,
    thread_pending_permissions,
  ]);

  useEffect(() => {
    set_thread_panel_data(thread_panel_data);
  }, [set_thread_panel_data, thread_panel_data]);
}
