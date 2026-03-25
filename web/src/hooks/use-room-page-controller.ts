"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { initialOptions } from "@/config/options";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { getRoom, getRoomContexts } from "@/lib/room-api";
import { useHomeWorkspaceController } from "@/hooks/use-home-workspace-controller";
import { useAgentStore } from "@/store/agent";
import { useConversationStore } from "@/store/conversation";
import { Agent, AgentOptions } from "@/types/agent";
import { Conversation, ConversationSnapshotPayload } from "@/types/conversation";
import { RoomContextAggregate } from "@/types/room";
import { RoomPageControllerOptions } from "@/types/route";

function to_timestamp(value?: string | null): number {
  if (!value) {
    return 0;
  }

  const parsed = new Date(value).getTime();
  return Number.isFinite(parsed) ? parsed : 0;
}

export function useRoomPageController({
  room_id,
  conversation_id,
}: RoomPageControllerOptions) {
  const {
    agents,
    create_agent,
    update_agent,
    delete_agent,
    load_agents_from_server,
  } = useAgentStore();
  const {
    conversations,
    load_conversations_from_server,
  } = useConversationStore();

  const [is_bootstrapped, set_is_bootstrapped] = useState(false);
  const [room_contexts, set_room_contexts] = useState<RoomContextAggregate[]>([]);
  const [is_room_loading, set_is_room_loading] = useState(false);
  const [room_error, set_room_error] = useState<string | null>(null);
  const [selected_member_agent_id, set_selected_member_agent_id] = useState<string | null>(null);
  const [is_dialog_open, set_is_dialog_open] = useState(false);
  const [dialog_mode, set_dialog_mode] = useState<"create" | "edit">("create");
  const [editing_agent_id, set_editing_agent_id] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    const bootstrap = async () => {
      try {
        await Promise.all([
          load_agents_from_server(),
          load_conversations_from_server(),
        ]);
      } finally {
        if (!cancelled) {
          set_is_bootstrapped(true);
        }
      }
    };

    void bootstrap();

    return () => {
      cancelled = true;
    };
  }, [load_agents_from_server, load_conversations_from_server]);

  useEffect(() => {
    if (!room_id) {
      set_room_contexts([]);
      set_room_error(null);
      set_is_room_loading(false);
      return;
    }

    let cancelled = false;
    set_is_room_loading(true);
    set_room_error(null);

    const load_room_context = async () => {
      try {
        const [room, contexts] = await Promise.all([
          getRoom(room_id),
          getRoomContexts(room_id),
        ]);

        if (cancelled) {
          return;
        }

        set_room_contexts(
          contexts.length
            ? contexts
            : [
              {
                room: room.room,
                members: room.members,
                conversation: {
                  id: "",
                  room_id: room.room.id,
                  conversation_type: "room_main",
                  title: room.room.name ?? "",
                },
                sessions: [],
              },
            ],
        );
      } catch (error) {
        if (cancelled) {
          return;
        }
        set_room_contexts([]);
        set_room_error(error instanceof Error ? error.message : "加载 room 失败");
      } finally {
        if (!cancelled) {
          set_is_room_loading(false);
        }
      }
    };

    void load_room_context();

    return () => {
      cancelled = true;
    };
  }, [room_id]);

  const current_room = useMemo(
    () => room_contexts[0]?.room ?? null,
    [room_contexts],
  );

  const room_member_agents = useMemo(() => {
    const agent_ids = new Set(
      room_contexts[0]?.members
        .filter((member) => member.member_type === "agent")
        .map((member) => member.member_agent_id)
        .filter((member_agent_id): member_agent_id is string => Boolean(member_agent_id)) ?? [],
    );

    return agents.filter((agent) => agent_ids.has(agent.agent_id));
  }, [agents, room_contexts]);

  const room_conversations = useMemo<Conversation[]>(() => {
    return room_contexts
      .filter((context) => Boolean(context.conversation.id))
      .map((context) => {
        const session_conversations = conversations.filter(
          (conversation) =>
            conversation.room_id === context.room.id &&
            conversation.conversation_id === context.conversation.id,
        );
        const latest_conversation = [...session_conversations].sort(
          (left, right) => right.last_activity_at - left.last_activity_at,
        )[0];

        return {
          session_key: context.conversation.id,
          room_id: context.room.id,
          conversation_id: context.conversation.id,
          session_id: latest_conversation?.session_id ?? null,
          agent_id: latest_conversation?.agent_id ?? context.sessions[0]?.agent_id,
          title: context.conversation.title?.trim() || context.room.name || "未命名对话",
          options: latest_conversation?.options ?? {},
          created_at:
            latest_conversation?.created_at ?? to_timestamp(context.conversation.created_at),
          last_activity_at:
            latest_conversation?.last_activity_at ?? to_timestamp(context.conversation.updated_at),
          is_active: latest_conversation?.is_active,
          message_count:
            latest_conversation?.message_count ??
            session_conversations.reduce(
              (count, conversation) => count + (conversation.message_count ?? 0),
              0,
            ),
        };
      })
      .sort((left, right) => right.last_activity_at - left.last_activity_at);
  }, [conversations, room_contexts]);

  const current_room_conversation_id = useMemo(
    () => conversation_id ?? room_conversations[0]?.session_key ?? null,
    [conversation_id, room_conversations],
  );

  const current_room_context = useMemo(
    () =>
      room_contexts.find((context) => context.conversation.id === current_room_conversation_id) ??
      room_contexts[0] ??
      null,
    [current_room_conversation_id, room_contexts],
  );

  useEffect(() => {
    const next_member_ids =
      current_room_context?.sessions
        .map((session) => session.agent_id)
        .filter(Boolean) ?? [];

    if (!next_member_ids.length) {
      set_selected_member_agent_id(null);
      return;
    }

    if (
      !selected_member_agent_id ||
      !next_member_ids.includes(selected_member_agent_id)
    ) {
      set_selected_member_agent_id(next_member_ids[0]);
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

  const current_conversation = useMemo(
    () =>
      conversations.find(
        (conversation) => conversation.session_id === active_room_session?.id,
      ) ?? null,
    [active_room_session?.id, conversations],
  );

  const current_agent = useMemo(
    () =>
      room_member_agents.find(
        (agent) => agent.agent_id === active_room_session?.agent_id,
      ) ?? null,
    [active_room_session?.agent_id, room_member_agents],
  );

  const editing_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === editing_agent_id) ?? null,
    [agents, editing_agent_id],
  );

  const dialog_initial_title = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.name : undefined),
    [dialog_mode, editing_agent?.name],
  );

  const dialog_initial_options = useMemo(() => {
    if (dialog_mode !== "edit" || !editing_agent) {
      return initialOptions;
    }

    return {
      model: editing_agent.options.model,
      permission_mode: editing_agent.options.permission_mode,
      allowed_tools: editing_agent.options.allowed_tools,
      disallowed_tools: editing_agent.options.disallowed_tools,
      max_turns: editing_agent.options.max_turns,
      max_thinking_tokens: editing_agent.options.max_thinking_tokens,
      skills_enabled: editing_agent.options.skills_enabled,
      setting_sources: editing_agent.options.setting_sources,
    };
  }, [dialog_mode, editing_agent]);

  const workspace = useHomeWorkspaceController({
    current_agent_id: current_agent?.agent_id ?? null,
    current_conversation,
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

  const handle_open_create_agent = useCallback(() => {
    set_dialog_mode("create");
    set_editing_agent_id(null);
    set_is_dialog_open(true);
  }, []);

  const handle_edit_agent = useCallback((agent_id: string) => {
    set_dialog_mode("edit");
    set_editing_agent_id(agent_id);
    set_is_dialog_open(true);
  }, []);

  const handle_delete_agent = useCallback(async (agent_id: string) => {
    await delete_agent(agent_id);
  }, [delete_agent]);

  const handle_save_agent_options = useCallback(async (title: string, options: AgentOptions) => {
    const next_options = {
      model: options.model,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      skills_enabled: options.skills_enabled,
      setting_sources: options.setting_sources,
    };

    if (dialog_mode === "create") {
      await create_agent({
        name: title,
        options: next_options,
      });
      return;
    }

    if (dialog_mode === "edit" && editing_agent_id) {
      await update_agent(editing_agent_id, {
        name: title,
        options: next_options,
      });
    }
  }, [create_agent, dialog_mode, editing_agent_id, update_agent]);

  const handle_validate_agent_name = useCallback(async (name: string) => {
    const exclude_agent_id = dialog_mode === "edit" ? editing_agent_id ?? undefined : undefined;
    return validateAgentNameApi(name, exclude_agent_id);
  }, [dialog_mode, editing_agent_id]);

  const handle_conversation_snapshot_change = useCallback((_snapshot: ConversationSnapshotPayload) => {
    // room 页当前直接依赖后端 room 上下文与真实 session 列表，不再在本地伪造 thread 快照。
  }, []);

  const is_hydrated = is_bootstrapped && !is_room_loading;

  return {
    agents,
    conversations,
    room_error,
    current_room,
    current_room_title: current_room?.name?.trim() || current_agent?.name || "未命名 room",
    current_room_description: current_room?.description ?? "",
    room_members: room_member_agents,
    current_agent,
    current_agent_id: current_agent?.agent_id ?? null,
    current_room_conversations: room_conversations,
    current_conversation,
    current_conversation_id: current_room_conversation_id,
    recent_agents: room_member_agents,
    is_hydrated,
    is_dialog_open,
    dialog_mode,
    dialog_initial_title,
    dialog_initial_options,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_select_agent,
    handle_select_conversation,
    handle_back_to_directory,
    handle_delete_agent,
    handle_create_conversation: async () => {},
    handle_save_agent_options,
    handle_validate_agent_name,
    handle_open_conversation_from_launcher: () => {},
    handle_conversation_snapshot_change,
    handle_delete_conversation: async () => {},
    route_conversation_id: conversation_id ?? null,
    route_room_id: room_id ?? null,
    ...workspace,
  };
}
