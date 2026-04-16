"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { is_main_agent } from "@/config/options";
import {
  add_room_member,
  create_room_conversation,
  delete_room,
  delete_room_conversation,
  remove_room_member,
  update_room,
  update_room_conversation,
} from "@/lib/api/room-api";
import { useHomeWorkspaceController } from "@/hooks/home/use-home-workspace-controller";
import {
  build_room_conversation_views,
  resolve_current_agent_session_identity,
  resolve_current_room_context,
  resolve_room_member_agents,
  resolve_selected_conversation_id,
  resolve_selected_member_agent_id,
} from "@/hooks/room-page-controller/room-page-controller-core";
import { useRoomPageAgentDialog } from "@/hooks/room-page-controller/use-room-page-agent-dialog";
import { useRoomPageData } from "@/hooks/room-page-controller/use-room-page-data";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { UpdateRoomParams } from "@/types/conversation/room";
import { RoomPageControllerOptions } from "@/types/app/route";

export function useRoomPageController({
  room_id,
  conversation_id,
}: RoomPageControllerOptions) {
  // 这里坚持使用细粒度 selector，避免 Room 页面因为 store
  // 里无关字段变动而整页重渲染。
  const agents = useAgentStore((s) => s.agents);
  const create_agent = useAgentStore((s) => s.create_agent);
  const update_agent = useAgentStore((s) => s.update_agent);
  const delete_agent = useAgentStore((s) => s.delete_agent);
  const load_agents_from_server = useAgentStore((s) => s.load_agents_from_server);

  const conversations = useConversationStore((s) => s.conversations);
  const sync_conversation_snapshot = useConversationStore((s) => s.sync_conversation_snapshot);
  const load_conversations_from_server = useConversationStore((s) => s.load_conversations_from_server);

  const [selected_member_agent_id, set_selected_member_agent_id] = useState<string | null>(null);
  const {
    is_bootstrapped,
    room_contexts,
    set_room_contexts,
    rooms,
    room_error,
    is_room_loading,
    refresh_rooms,
    refresh_room_contexts,
  } = useRoomPageData({
    room_id,
    load_agents_from_server,
    load_conversations_from_server,
  });
  const {
    is_dialog_open,
    dialog_mode,
    editing_agent_id,
    dialog_initial_title,
    dialog_initial_avatar,
    dialog_initial_description,
    dialog_initial_options,
    dialog_initial_vibe_tags,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_save_agent_options,
    handle_save_existing_agent_options,
    handle_validate_agent_name,
    handle_validate_agent_name_for_agent,
  } = useRoomPageAgentDialog({
    agents,
    create_agent,
    update_agent,
  });

  const scoped_room_contexts = useMemo(
    () => room_contexts.filter((context) => context.room.id === room_id),
    [room_contexts, room_id],
  );

  const current_room = useMemo(
    () => scoped_room_contexts[0]?.room ?? null,
    [scoped_room_contexts],
  );

  const room_member_agents = useMemo(() => {
    return resolve_room_member_agents(agents, scoped_room_contexts);
  }, [agents, scoped_room_contexts]);

  const room_conversations = useMemo<RoomConversationView[]>(() => {
    return build_room_conversation_views(scoped_room_contexts, conversations);
  }, [conversations, scoped_room_contexts]);

  const selected_conversation_id = useMemo(() => {
    return resolve_selected_conversation_id(conversation_id, room_conversations);
  }, [conversation_id, room_conversations]);

  const current_room_conversation = useMemo(
    () =>
      room_conversations.find(
        (conversation) => conversation.conversation_id === selected_conversation_id,
      ) ?? null,
    [room_conversations, selected_conversation_id],
  );

  const current_room_context = useMemo(
    () => resolve_current_room_context(scoped_room_contexts, selected_conversation_id),
    [scoped_room_contexts, selected_conversation_id],
  );

  useEffect(() => {
    const next_selected_member_agent_id = resolve_selected_member_agent_id(
      current_room_context,
      selected_member_agent_id,
    );

    if (selected_member_agent_id !== next_selected_member_agent_id) {
      set_selected_member_agent_id(next_selected_member_agent_id);
    }
  }, [current_room_context, selected_member_agent_id]);

  const active_room_session = useMemo(
    () =>
      current_room_context?.sessions.find(
        (session) => session.agent_id === selected_member_agent_id,
      ) ??
      current_room_context?.sessions[0] ??
      null,
    [current_room_context, selected_member_agent_id],
  );

  const current_agent_conversation = useMemo(
    () =>
      conversations.find(
        (conversation) => conversation.room_session_id === active_room_session?.id,
      ) ?? null,
    [active_room_session?.id, conversations],
  );

  // DM 模式下，store 里可能暂时没有对应 conversation，
  // 这里直接回退到 room session，避免 workspace loader 被空 session_key 卡住。
  const current_agent_session_identity = useMemo(() => {
    return resolve_current_agent_session_identity({
      current_agent_conversation,
      current_room_id: current_room?.id ?? null,
      active_room_session,
      current_room_type: current_room?.room_type ?? "dm",
    });
  }, [active_room_session, current_agent_conversation, current_room?.id, current_room?.room_type]);

  const current_agent = useMemo(
    () =>
      room_member_agents.find(
        (agent) => agent.agent_id === active_room_session?.agent_id,
      ) ?? null,
    [active_room_session?.agent_id, room_member_agents],
  );
  const available_room_agents = useMemo(() => {
    const joined_agent_ids = new Set(room_member_agents.map((agent) => agent.agent_id));
    return agents.filter((agent) => (
      !joined_agent_ids.has(agent.agent_id) &&
      !is_main_agent(agent.agent_id)
    ));
  }, [agents, room_member_agents]);

  const workspace = useHomeWorkspaceController({
    current_agent_id: current_agent?.agent_id ?? null,
    current_agent_conversation,
    current_agent_session_identity,
  });

  const handle_select_agent = useCallback((agent_id: string) => {
    set_selected_member_agent_id(agent_id);
  }, []);

  const handle_select_conversation = useCallback((_next_conversation_id: string) => {
    // 路由层负责切换当前 room conversation。
  }, []);

  const handle_back_to_directory = useCallback(() => {
    set_selected_member_agent_id(null);
  }, []);

  const handle_delete_agent = useCallback(async (agent_id: string) => {
    await delete_agent(agent_id);
  }, [delete_agent]);

  const handle_conversation_snapshot_change = useCallback((snapshot: ConversationSnapshotPayload) => {
    const snapshot_conversation_id = "conversation_id" in snapshot
      ? snapshot.conversation_id
      : current_room_context?.conversation.id ?? null;
    const snapshot_room_session_id = "room_session_id" in snapshot
      ? snapshot.room_session_id ?? null
      : active_room_session?.id ?? null;
    const next_last_activity_at = snapshot.last_activity_at
      ? new Date(snapshot.last_activity_at).toISOString()
      : undefined;

    set_room_contexts((prev) => {
      if (!snapshot_conversation_id) {
        return prev;
      }

      let has_changed = false;

      const next_contexts = prev.map((context) => {
        if (context.conversation.id !== snapshot_conversation_id) {
          return context;
        }

        let context_changed = false;
        const next_sessions = context.sessions.map((session) => {
          if (!snapshot_room_session_id || session.id !== snapshot_room_session_id) {
            return session;
          }

          const next_sdk_session_id = snapshot.session_id ?? session.sdk_session_id;
          const next_session_last_activity_at = next_last_activity_at ?? session.last_activity_at;
          const session_changed =
            session.sdk_session_id !== next_sdk_session_id ||
            session.last_activity_at !== next_session_last_activity_at;

          if (!session_changed) {
            return session;
          }

          has_changed = true;
          context_changed = true;
          return {
            ...session,
            sdk_session_id: next_sdk_session_id,
            last_activity_at: next_session_last_activity_at,
          };
        });

        if (!context_changed) {
          return context;
        }

        return {
          ...context,
          sessions: next_sessions,
        };
      });

      return has_changed ? next_contexts : prev;
    });

    const snapshot_session_key = "session_key" in snapshot
      ? snapshot.session_key
      : current_agent_session_identity?.session_key ?? null;

    if (!snapshot_session_key) {
      return;
    }

    const snapshot_conversation = conversations.find((conversation) => (
      (snapshot_room_session_id && conversation.room_session_id === snapshot_room_session_id) ||
      conversation.session_key === snapshot_session_key
    )) ?? null;

    const next_snapshot = {
      message_count: snapshot.message_count,
      ...(snapshot.last_activity_at ? { last_activity_at: snapshot.last_activity_at } : {}),
      session_id: snapshot.session_id,
    };

    const conversation_changed =
      !snapshot_conversation ||
      snapshot_conversation.message_count !== next_snapshot.message_count ||
      (next_snapshot.last_activity_at !== undefined &&
        snapshot_conversation.last_activity_at !== next_snapshot.last_activity_at) ||
      snapshot_conversation.session_id !== next_snapshot.session_id;

    if (!conversation_changed) {
      return;
    }

    sync_conversation_snapshot(snapshot_session_key, next_snapshot);
  }, [
    active_room_session?.id,
    current_room_context?.conversation.id,
    conversations,
    current_agent_session_identity?.session_key,
    set_room_contexts,
    sync_conversation_snapshot,
  ]);

  const handle_update_room = useCallback(async (params: UpdateRoomParams) => {
    if (!room_id) {
      return;
    }
    await update_room(room_id, params);
    await refresh_rooms();
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
  }, [load_conversations_from_server, refresh_room_contexts, refresh_rooms, room_id]);

  const handle_delete_room = useCallback(async () => {
    if (!room_id) {
      return;
    }
    await delete_room(room_id);
    await refresh_rooms();
  }, [refresh_rooms, room_id]);

  const handle_create_conversation = useCallback(async (title?: string) => {
    if (!room_id) {
      return null;
    }
    const context = await create_room_conversation(room_id, {title});
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
    return context.conversation.id;
  }, [load_conversations_from_server, refresh_room_contexts, room_id]);

  const handle_delete_conversation = useCallback(async (conversation_id: string) => {
    if (!room_id) {
      return null;
    }
    const fallback_context = await delete_room_conversation(room_id, conversation_id);
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
    return fallback_context.conversation.id;
  }, [load_conversations_from_server, refresh_room_contexts, room_id]);

  const handle_update_conversation_title = useCallback(async (conversation_id: string, title: string) => {
    if (!room_id) return;
    await update_room_conversation(room_id, conversation_id, { title });
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
  }, [load_conversations_from_server, refresh_room_contexts, room_id]);

  const handle_add_room_member = useCallback(async (agent_id: string) => {
    if (!room_id) {
      return;
    }
    await add_room_member(room_id, agent_id);
    await refresh_rooms();
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
  }, [load_conversations_from_server, refresh_room_contexts, refresh_rooms, room_id]);

  const handle_remove_room_member = useCallback(async (agent_id: string) => {
    if (!room_id) {
      return;
    }
    await remove_room_member(room_id, agent_id);
    await refresh_rooms();
    await refresh_room_contexts(room_id);
    await load_conversations_from_server();
  }, [load_conversations_from_server, refresh_room_contexts, refresh_rooms, room_id]);

  const handle_open_conversation_from_launcher = useCallback((conversation_id: string, agent_id?: string) => {
    // Launcher 打开 Room 时只认 conversation_id，不再接受其他回退标识。
    const target_conversation = room_conversations.find(
      (conversation) => conversation.conversation_id === conversation_id,
    );

    if (!target_conversation) {
      return;
    }

    // 如果指定了 agent_id，优先使用
    // 否则使用 conversation 的 agent_id
    const target_agent_id = agent_id ?? target_conversation.agent_id ?? null;

    if (target_agent_id && room_member_agents.some((agent) => agent.agent_id === target_agent_id)) {
      set_selected_member_agent_id(target_agent_id);
    } else if (room_member_agents.length > 0) {
      // 如果指定的 agent 不在当前 room 中，默认选择第一个
      set_selected_member_agent_id(room_member_agents[0].agent_id);
    }
  }, [room_conversations, room_member_agents]);

  const handle_refresh_room_state = useCallback(async () => {
    if (!room_id) {
      return;
    }

    await Promise.all([
      refresh_room_contexts(room_id),
      load_conversations_from_server(),
    ]);
  }, [load_conversations_from_server, refresh_room_contexts, room_id]);

  const is_hydrated = is_bootstrapped && !is_room_loading;

  // 对外 controller 对象本身保持稳定，避免消费端因为对象引用变化
  // 产生无意义重渲染。
  return useMemo(() => ({
    agents,
    conversations,
    rooms,
    room_error,
    current_room,
    current_room_type: current_room?.room_type ?? "room",
    current_room_title: current_room?.name?.trim() || current_agent?.name || "未命名 room",
    current_room_description: current_room?.description ?? "",
    room_members: room_member_agents,
    available_room_agents,
    current_agent,
    current_agent_id: current_agent?.agent_id ?? null,
    current_room_conversations: room_conversations,
    current_room_conversation,
    current_agent_conversation,
    current_agent_session_identity,
    conversation_id: selected_conversation_id,
    recent_agents: room_member_agents,
    is_hydrated,
    is_dialog_open,
    dialog_mode,
    editing_agent_id,
    dialog_initial_title,
    dialog_initial_avatar,
    dialog_initial_description,
    dialog_initial_options,
    dialog_initial_vibe_tags,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_select_agent,
    handle_select_conversation,
    handle_back_to_directory,
    handle_delete_agent,
    handle_create_conversation,
    handle_save_agent_options,
    handle_save_existing_agent_options,
    handle_validate_agent_name,
    handle_validate_agent_name_for_agent,
    handle_open_conversation_from_launcher,
    handle_refresh_room_state,
    handle_conversation_snapshot_change,
    handle_delete_conversation,
    handle_update_conversation_title,
    handle_update_room,
    handle_delete_room,
    handle_add_room_member,
    handle_remove_room_member,
    route_room_id: room_id ?? null,
    ...workspace,
  }), [
    agents, conversations, rooms, room_error, current_room, current_agent,
    room_member_agents, available_room_agents, room_conversations, current_room_conversation, current_agent_conversation,
    current_agent_session_identity, selected_conversation_id, is_hydrated, is_dialog_open, dialog_mode,
    editing_agent_id, dialog_initial_title, dialog_initial_avatar, dialog_initial_description, dialog_initial_options, dialog_initial_vibe_tags, set_is_dialog_open,
    handle_open_create_agent, handle_edit_agent, handle_select_agent,
    handle_select_conversation, handle_back_to_directory, handle_delete_agent,
    handle_create_conversation, handle_save_agent_options, handle_save_existing_agent_options, handle_validate_agent_name, handle_validate_agent_name_for_agent,
    handle_open_conversation_from_launcher, handle_refresh_room_state, handle_conversation_snapshot_change,
    handle_delete_conversation, handle_update_conversation_title, handle_update_room, handle_delete_room,
    handle_add_room_member, handle_remove_room_member, room_id, workspace,
  ]);
}
