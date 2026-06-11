"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { is_main_agent } from "@/config/options";
import { get_agent_sessions_api } from "@/lib/api/agent-api";
import {
  add_room_member,
  close_room_conversation_runtime,
  create_room_conversation,
  delete_room,
  delete_room_conversation,
  notify_room_directory_updated,
  remove_room_member,
  subscribe_room_directory_updates,
  update_room,
  update_room_conversation,
} from "@/lib/api/room-api";
import {
  build_external_session_conversation_id,
  format_external_session_title,
  is_external_session_channel,
} from "@/features/conversation/external-session-labels";
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
import { AgentIdentityDraft, AgentOptions, AgentSession } from "@/types/agent/agent";
import { AgentConversationIdentity } from "@/types/agent/agent-conversation";
import { ConversationSnapshotPayload, RoomConversationView } from "@/types/conversation/conversation";
import { UpdateRoomParams } from "@/types/conversation/room";
import { RoomPageControllerOptions } from "@/types/app/route";

const EXTERNAL_AGENT_SESSION_REFRESH_INTERVAL_MS = 8000;

function build_external_room_conversation_views({
  room_id,
  sessions,
}: {
  room_id: string | null;
  sessions: AgentSession[];
}): RoomConversationView[] {
  if (!room_id) {
    return [];
  }
  return sessions
    .filter((session) => (
      !session.room_id &&
      is_external_session_channel(session.channel_type, session.session_key)
    ))
    .map((session) => ({
      session_key: session.session_key,
      room_id,
      conversation_id: build_external_session_conversation_id(session.session_key),
      conversation_type: "external",
      session_id: session.session_id,
      agent_id: session.agent_id,
      title: format_external_session_title({
        channel_type: session.channel_type,
        session_key: session.session_key,
        title: session.title,
      }),
      options: {
        channel_type: session.channel_type,
        chat_type: session.chat_type,
        external_session: true,
      },
      created_at: session.created_at,
      last_activity_at: session.last_activity_at,
      is_active: session.status === "active",
      message_count: session.message_count,
    }))
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

function are_external_agent_sessions_equal(left: AgentSession[], right: AgentSession[]): boolean {
  if (left.length !== right.length) {
    return false;
  }
  return left.every((item, index) => {
    const other = right[index];
    return other !== undefined &&
      item.session_key === other.session_key &&
      item.status === other.status &&
      item.message_count === other.message_count &&
      item.last_activity_at === other.last_activity_at &&
      item.title === other.title &&
      item.channel_type === other.channel_type &&
      item.chat_type === other.chat_type;
  });
}

function filter_external_agent_sessions(sessions: AgentSession[]): AgentSession[] {
  return sessions
    .filter((item) => (
      !item.room_id &&
      is_external_session_channel(item.channel_type, item.session_key)
    ))
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function useRoomPageController({
  room_id,
  conversation_id,
  session_key,
}: RoomPageControllerOptions) {
  // 这里坚持使用细粒度 selector，避免 Room 页面因为 store
  // 里无关字段变动而整页重渲染。
  const agents = useAgentStore((s) => s.agents);
  const create_agent = useAgentStore((s) => s.create_agent);
  const update_agent = useAgentStore((s) => s.update_agent);
  const delete_agent = useAgentStore((s) => s.delete_agent);
  const load_agents_from_server = useAgentStore((s) => s.load_agents_from_server);

  const sync_conversation_snapshot = useConversationStore((s) => s.sync_conversation_snapshot);

  const [selected_member_agent_id, set_selected_member_agent_id] = useState<string | null>(null);
  const [external_agent_sessions, set_external_agent_sessions] = useState<AgentSession[]>([]);
  const [external_session_refresh_version, set_external_session_refresh_version] = useState(0);
  const {
    is_bootstrapped,
    room_contexts,
    set_room_contexts,
    room_error,
    is_room_loading,
    refresh_room_contexts,
  } = useRoomPageData({
    room_id,
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
    return resolve_room_member_agents(scoped_room_contexts);
  }, [scoped_room_contexts]);

  const workspace_agent_ids = useMemo(() => {
    return room_member_agents.map((agent) => agent.agent_id);
  }, [room_member_agents]);

  const base_room_conversations = useMemo<RoomConversationView[]>(() => {
    return build_room_conversation_views(scoped_room_contexts);
  }, [scoped_room_contexts]);
  const route_session_key = useMemo(
    () => session_key?.trim() || null,
    [session_key],
  );

  const selected_base_conversation_id = useMemo(() => {
    return resolve_selected_conversation_id(conversation_id, base_room_conversations);
  }, [base_room_conversations, conversation_id]);

  const current_room_context = useMemo(
    () => resolve_current_room_context(scoped_room_contexts, selected_base_conversation_id),
    [scoped_room_contexts, selected_base_conversation_id],
  );

  const external_room_conversations = useMemo(
    () => build_external_room_conversation_views({
      room_id: current_room?.id ?? null,
      sessions: external_agent_sessions,
    }),
    [current_room?.id, external_agent_sessions],
  );

  const current_room_conversations = useMemo(
    () => [...base_room_conversations, ...external_room_conversations]
      .sort((left, right) => right.last_activity_at - left.last_activity_at),
    [base_room_conversations, external_room_conversations],
  );

  const selected_conversation_id = useMemo(() => {
    if (route_session_key) {
      return build_external_session_conversation_id(route_session_key);
    }
    return selected_base_conversation_id;
  }, [route_session_key, selected_base_conversation_id]);

  const current_room_conversation = useMemo(
    () =>
      current_room_conversations.find(
        (conversation) => conversation.conversation_id === selected_conversation_id,
      ) ?? null,
    [current_room_conversations, selected_conversation_id],
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

  const current_agent = useMemo(
    () =>
      room_member_agents.find(
        (agent) => agent.agent_id === active_room_session?.agent_id,
      ) ?? null,
    [active_room_session?.agent_id, room_member_agents],
  );

  useEffect(
    () => subscribe_room_directory_updates(() => {
      set_external_session_refresh_version((version) => version + 1);
    }),
    [],
  );

  useEffect(() => {
    if (current_room?.room_type !== "dm" || !current_agent?.agent_id) {
      set_external_agent_sessions([]);
      return undefined;
    }

    let cancelled = false;
    const refresh_external_sessions = () => {
      void get_agent_sessions_api(current_agent.agent_id)
        .then((sessions) => {
          if (cancelled) {
            return;
          }
          const next_sessions = filter_external_agent_sessions(sessions);
          set_external_agent_sessions((current_sessions) => (
            are_external_agent_sessions_equal(current_sessions, next_sessions)
              ? current_sessions
              : next_sessions
          ));
        })
        .catch((error) => {
          console.error("[RoomPage] 加载 Agent 外部 IM 会话失败:", error);
          if (!cancelled) {
            set_external_agent_sessions([]);
          }
        });
    };
    const refresh_if_visible = () => {
      if (cancelled) {
        return;
      }
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return;
      }
      refresh_external_sessions();
    };

    refresh_external_sessions();
    const interval_id = window.setInterval(refresh_if_visible, EXTERNAL_AGENT_SESSION_REFRESH_INTERVAL_MS);
    window.addEventListener("focus", refresh_if_visible);
    document.addEventListener("visibilitychange", refresh_if_visible);

    return () => {
      cancelled = true;
      window.clearInterval(interval_id);
      window.removeEventListener("focus", refresh_if_visible);
      document.removeEventListener("visibilitychange", refresh_if_visible);
    };
  }, [current_agent?.agent_id, current_room?.room_type, external_session_refresh_version]);

  // Room 详情页现在直接基于当前 room context 解析 session 身份；
  // 外部 IM 会话则以 route session_key 作为同一 Agent 下的独立会话。
  const current_agent_session_identity = useMemo<AgentConversationIdentity | null>(() => {
    if (route_session_key && current_agent?.agent_id) {
      const external_session = external_agent_sessions.find((item) => item.session_key === route_session_key);
      const external_chat_type: AgentConversationIdentity["chat_type"] =
        external_session?.chat_type === "group" ? "group" : "dm";
      return {
        session_key: route_session_key,
        agent_id: external_session?.agent_id ?? current_agent.agent_id,
        chat_type: external_chat_type,
      };
    }

    return resolve_current_agent_session_identity({
      current_room_id: current_room?.id ?? null,
      current_conversation_id: current_room_context?.conversation.id ?? null,
      active_room_session,
      current_room_type: current_room?.room_type ?? "dm",
    });
  }, [
    active_room_session,
    current_agent?.agent_id,
    current_room?.id,
    current_room?.room_type,
    current_room_context?.conversation.id,
    external_agent_sessions,
    route_session_key,
  ]);
  const available_room_agents = useMemo(() => {
    const joined_agent_ids = new Set(room_member_agents.map((agent) => agent.agent_id));
    return agents.filter((agent) => (
      !joined_agent_ids.has(agent.agent_id) &&
      !is_main_agent(agent.agent_id)
    ));
  }, [agents, room_member_agents]);

  const handle_prepare_room_agent_catalog = useCallback(async () => {
    await load_agents_from_server();
  }, [load_agents_from_server]);

  const workspace = useHomeWorkspaceController({
    current_agent_id: current_agent?.agent_id ?? null,
    workspace_agent_ids,
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

    const next_snapshot = {
      ...(snapshot.last_activity_at ? { last_activity_at: snapshot.last_activity_at } : {}),
      session_id: snapshot.session_id,
    };

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
        const next_conversation_updated_at = next_last_activity_at ?? context.conversation.updated_at;
        const conversation_changed =
          context.conversation.updated_at !== next_conversation_updated_at;

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
          if (!conversation_changed) {
            return context;
          }
        }

        has_changed = true;
        return {
          ...context,
          conversation: {
            ...context.conversation,
            updated_at: next_conversation_updated_at,
          },
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

    sync_conversation_snapshot(snapshot_session_key, next_snapshot);
    if (is_external_session_channel(null, snapshot_session_key)) {
      notify_room_directory_updated();
    }
  }, [
    active_room_session?.id,
    current_room_context?.conversation.id,
    current_agent_session_identity?.session_key,
    set_room_contexts,
    sync_conversation_snapshot,
  ]);

  const handle_update_room = useCallback(async (params: UpdateRoomParams) => {
    if (!room_id) {
      return;
    }
    await update_room(room_id, params);
    await refresh_room_contexts(room_id);
  }, [refresh_room_contexts, room_id]);

  const handle_delete_room = useCallback(async () => {
    if (!room_id) {
      return;
    }
    await delete_room(room_id);
  }, [room_id]);

  const handle_create_conversation = useCallback(async (title?: string) => {
    if (!room_id) {
      return null;
    }
    const context = await create_room_conversation(room_id, {title});
    await refresh_room_contexts(room_id);
    return context.conversation.id;
  }, [refresh_room_contexts, room_id]);

  const handle_delete_conversation = useCallback(async (conversation_id: string) => {
    if (!room_id) {
      return null;
    }
    const fallback_context = await delete_room_conversation(room_id, conversation_id);
    await refresh_room_contexts(room_id);
    return fallback_context.conversation.id;
  }, [refresh_room_contexts, room_id]);

  const handle_close_conversation = useCallback(async (conversation_id: string) => {
    if (!room_id) {
      return;
    }
    await close_room_conversation_runtime(room_id, conversation_id);
  }, [room_id]);

  const handle_update_conversation_title = useCallback(async (conversation_id: string, title: string) => {
    if (!room_id) return;
    await update_room_conversation(room_id, conversation_id, { title });
    await refresh_room_contexts(room_id);
  }, [refresh_room_contexts, room_id]);

  const handle_add_room_member = useCallback(async (agent_id: string) => {
    if (!room_id) {
      return;
    }
    await add_room_member(room_id, agent_id);
    await refresh_room_contexts(room_id);
  }, [refresh_room_contexts, room_id]);

  const handle_save_existing_room_member_options = useCallback(async (
    agent_id: string,
    title: string,
    options: AgentOptions,
    identity: AgentIdentityDraft,
  ) => {
    await handle_save_existing_agent_options(agent_id, title, options, identity);
    if (!room_id) {
      return;
    }
    await refresh_room_contexts(room_id);
  }, [handle_save_existing_agent_options, refresh_room_contexts, room_id]);

  const handle_remove_room_member = useCallback(async (agent_id: string) => {
    if (!room_id) {
      return;
    }
    await remove_room_member(room_id, agent_id);
    await refresh_room_contexts(room_id);
  }, [refresh_room_contexts, room_id]);

  const handle_open_conversation_from_launcher = useCallback((conversation_id: string, agent_id?: string) => {
    // Launcher 打开 Room 时只认 conversation_id，不再接受其他回退标识。
    const target_conversation = current_room_conversations.find(
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
  }, [current_room_conversations, room_member_agents]);

  const handle_refresh_room_state = useCallback(async () => {
    if (!room_id) {
      return;
    }

    await refresh_room_contexts(room_id);
  }, [refresh_room_contexts, room_id]);

  const is_hydrated = is_bootstrapped && !is_room_loading;

  // 对外 controller 对象本身保持稳定，避免消费端因为对象引用变化
  // 产生无意义重渲染。
  return useMemo(() => ({
    agents,
    room_error,
    current_room,
    current_room_type: current_room?.room_type ?? "room",
    current_room_title: current_room?.name?.trim() || current_agent?.name || "未命名 room",
    current_room_description: current_room?.description ?? "",
    current_room_skill_names: current_room?.skill_names ?? [],
    room_members: room_member_agents,
    available_room_agents,
    handle_prepare_room_agent_catalog,
    current_agent,
    current_agent_id: current_agent?.agent_id ?? null,
    current_room_conversations,
    current_room_conversation,
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
    handle_save_existing_agent_options: handle_save_existing_room_member_options,
    handle_validate_agent_name,
    handle_validate_agent_name_for_agent,
    handle_open_conversation_from_launcher,
    handle_refresh_room_state,
    handle_conversation_snapshot_change,
    handle_close_conversation,
    handle_delete_conversation,
    handle_update_conversation_title,
    handle_update_room,
    handle_delete_room,
    handle_add_room_member,
    handle_remove_room_member,
    route_room_id: room_id ?? null,
    ...workspace,
  }), [
    agents, room_error, current_room, current_agent,
    room_member_agents, available_room_agents, current_room_conversations, current_room_conversation,
    current_agent_session_identity, selected_conversation_id, is_hydrated, is_dialog_open, dialog_mode,
    editing_agent_id, dialog_initial_title, dialog_initial_avatar, dialog_initial_description, dialog_initial_options, dialog_initial_vibe_tags, set_is_dialog_open,
    handle_open_create_agent, handle_edit_agent, handle_select_agent,
    handle_select_conversation, handle_back_to_directory, handle_delete_agent,
    handle_create_conversation, handle_save_agent_options, handle_save_existing_room_member_options, handle_validate_agent_name, handle_validate_agent_name_for_agent,
    handle_open_conversation_from_launcher, handle_refresh_room_state, handle_conversation_snapshot_change,
    handle_close_conversation, handle_delete_conversation, handle_update_conversation_title, handle_update_room, handle_delete_room,
    handle_add_room_member, handle_remove_room_member, handle_prepare_room_agent_catalog, room_id, workspace,
  ]);
}
