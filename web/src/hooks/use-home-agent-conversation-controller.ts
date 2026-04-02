"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { initialOptions, resolveAgentId } from "@/config/options";
import { useInitializeConversations } from "@/hooks/use-initialize-conversations";
import { validateAgentNameApi } from "@/lib/agent-manage-api";
import { useConversationStore } from "@/store/conversation";
import { useAgentStore } from "@/store/agent";
import { AgentOptions } from "@/types/agent";
import { SessionSnapshotPayload } from "@/types/conversation";

export function useHomeAgentConversationController() {
  const {
    agents,
    current_agent_id,
    create_agent,
    update_agent,
    delete_agent,
    set_current_agent,
    load_agents_from_server,
  } = useAgentStore();
  const {
    conversations,
    current_session_key,
    create_conversation,
    set_current_session_key,
    sync_conversation_snapshot,
    delete_conversation,
    load_conversations_from_server,
  } = useConversationStore();

  const [is_dialog_open, set_is_dialog_open] = useState(false);
  const [dialog_mode, set_dialog_mode] = useState<"create" | "edit">("create");
  const [editing_agent_id, set_editing_agent_id] = useState<string | null>(null);

  useEffect(() => {
    load_agents_from_server();
  }, [load_agents_from_server]);

  const is_hydrated = useInitializeConversations({
    load_conversations_from_server,
    set_current_session_key,
    auto_select_first: false,
    debug_name: "Page",
  });

  const current_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === current_agent_id) ?? null,
    [agents, current_agent_id],
  );

  const conversations_by_agent = useMemo(() => {
    const grouped = new Map<string, typeof conversations>();
    conversations.forEach((conversation) => {
      const owner = resolveAgentId(conversation.agent_id);
      const currentList = grouped.get(owner) ?? [];
      currentList.push(conversation);
      grouped.set(owner, currentList);
    });

    grouped.forEach((groupedSessions) => {
      groupedSessions.sort((left, right) => right.last_activity_at - left.last_activity_at);
    });

    return grouped;
  }, [conversations]);

  const current_agent_sessions = useMemo(() => {
    if (!current_agent_id) {
      return [];
    }
    return conversations_by_agent.get(current_agent_id) ?? [];
  }, [conversations_by_agent, current_agent_id]);

  const current_session = useMemo(
    () => current_agent_sessions.find((conversation) => conversation.session_key === current_session_key) ?? null,
    [current_agent_sessions, current_session_key],
  );

  const recent_agents = useMemo(() => agents.slice(0, 4), [agents]);
  const editing_agent = useMemo(
    () => agents.find((agent) => agent.agent_id === editing_agent_id),
    [agents, editing_agent_id],
  );
  const dialog_initial_title = useMemo(
    () => (dialog_mode === "edit" ? editing_agent?.name : undefined),
    [dialog_mode, editing_agent],
  );
  const dialog_initial_options = useMemo(() => {
    if (dialog_mode !== "edit") {
      return initialOptions;
    }

    return {
      model: editing_agent?.options?.model,
      permission_mode: editing_agent?.options?.permission_mode,
      allowed_tools: editing_agent?.options?.allowed_tools,
      disallowed_tools: editing_agent?.options?.disallowed_tools,
      max_turns: editing_agent?.options?.max_turns,
      max_thinking_tokens: editing_agent?.options?.max_thinking_tokens,
      setting_sources: editing_agent?.options?.setting_sources,
    };
  }, [dialog_mode, editing_agent]);

  useEffect(() => {
    if (!current_agent_id) {
      if (current_session_key !== null) {
        set_current_session_key(null);
      }
      return;
    }

    const has_selected_session = current_agent_sessions.some(
      (conversation) => conversation.session_key === current_session_key,
    );
    if (!has_selected_session) {
      set_current_session_key(current_agent_sessions[0]?.session_key ?? null);
    }
  }, [current_agent_id, current_session_key, current_agent_sessions, set_current_session_key]);

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

  const handle_select_agent = useCallback((agent_id: string) => {
    set_current_agent(agent_id);
  }, [set_current_agent]);

  const handle_back_to_directory = useCallback(() => {
    set_current_agent(null);
  }, [set_current_agent]);

  const handle_delete_agent = useCallback(async (agent_id: string) => {
    await delete_agent(agent_id);
  }, [delete_agent]);

  const handle_create_session = useCallback(async () => {
    if (!current_agent_id) {
      return;
    }

    const key = await create_conversation({
      title: "New Chat",
      agent_id: current_agent_id,
    });
    set_current_session_key(key);
  }, [create_conversation, current_agent_id, set_current_session_key]);

  const handle_save_agent_options = useCallback(async (title: string, options: AgentOptions) => {
    const agentOptions = {
      model: options.model,
      permission_mode: options.permission_mode,
      allowed_tools: options.allowed_tools,
      disallowed_tools: options.disallowed_tools,
      setting_sources: options.setting_sources,
    };

    if (dialog_mode === "create") {
      const agent_id = await create_agent({
        name: title,
        options: agentOptions,
      });
      set_current_agent(agent_id);
      return;
    }

    if (dialog_mode === "edit" && editing_agent_id) {
      await update_agent(editing_agent_id, {
        name: title,
        options: agentOptions,
      });
    }
  }, [create_agent, dialog_mode, editing_agent_id, set_current_agent, update_agent]);

  const handle_validate_agent_name = useCallback(async (name: string) => {
    const exclude_agent_id = dialog_mode === "edit" ? (editing_agent_id ?? undefined) : undefined;
    return validateAgentNameApi(name, exclude_agent_id);
  }, [dialog_mode, editing_agent_id]);

  const handle_select_session = useCallback((session_key: string) => {
    set_current_session_key(session_key);
  }, [set_current_session_key]);

  const handle_open_session_from_launcher = useCallback((session_key: string, agent_id?: string) => {
    if (agent_id) {
      set_current_agent(agent_id);
    }
    set_current_session_key(session_key);
  }, [set_current_session_key, set_current_agent]);

  const handle_conversation_snapshot_change = useCallback((snapshot: SessionSnapshotPayload) => {
    if (!snapshot.session_key) {
      return;
    }

    sync_conversation_snapshot(snapshot.session_key, {
      message_count: snapshot.message_count,
      ...(snapshot.last_activity_at ? {last_activity_at: snapshot.last_activity_at} : {}),
      session_id: snapshot.session_id,
    });
  }, [sync_conversation_snapshot]);

  const handle_delete_session = useCallback(async (session_key: string) => {
    await delete_conversation(session_key);
    if (current_session_key === session_key) {
      const remaining = current_agent_sessions.filter((conversation) => conversation.session_key !== session_key);
      set_current_session_key(remaining[0]?.session_key ?? null);
    }
  }, [current_agent_sessions, current_session_key, delete_conversation, set_current_session_key]);

  return {
    agents,
    current_agent,
    current_agent_id,
    current_agent_sessions,
    current_session,
    current_session_key,
    conversations,
    recent_agents,
    is_hydrated,
    is_dialog_open,
    dialog_mode,
    dialog_initial_title,
    dialog_initial_options,
    set_is_dialog_open,
    handle_open_create_agent,
    handle_edit_agent,
    handle_select_agent,
    handle_back_to_directory,
    handle_delete_agent,
    handle_create_session,
    handle_save_agent_options,
    handle_validate_agent_name,
    handle_select_session,
    handle_open_session_from_launcher,
    handle_conversation_snapshot_change,
    handle_delete_session,
  };
}
