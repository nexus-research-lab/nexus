"use client";

import { useEffect, useMemo } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { get_agents } from "@/lib/api/agent-manage-api";
import { get_agent_sessions_api } from "@/lib/api/agent-api";
import { get_room_contexts, list_rooms } from "@/lib/api/room-api";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";

import {
  build_room_session_selections,
  format_session_label,
} from "./scheduled-task-dialog-time";
import type {
  ScheduledTaskDialogLabelOption,
  ScheduledTaskDialogSessionOption,
  TargetType,
} from "./scheduled-task-dialog-types";

interface ResourceState<T> {
  error: string | null;
  items: T[];
  loading: boolean;
}

export function useScheduledTaskDialogData({
  is_open,
  target_type,
  selected_agent_id,
  selected_room_id,
}: {
  is_open: boolean;
  target_type: TargetType;
  selected_agent_id: string;
  selected_room_id: string;
}) {
  const should_load_agents = is_open;
  const should_load_rooms = is_open && target_type === "room";
  const should_load_agent_sessions = is_open && target_type === "agent" && Boolean(selected_agent_id);
  const should_load_room_contexts = is_open && target_type === "room" && Boolean(selected_room_id);
  const [agents_state, set_agents_state] = useResettableState<ResourceState<Agent>>(
    { error: null, items: [], loading: should_load_agents },
    is_open ? "open" : "closed",
  );
  const [agent_sessions_state, set_agent_sessions_state] = useResettableState<ResourceState<AgentSession>>(
    { error: null, items: [], loading: should_load_agent_sessions },
    `${is_open ? "open" : "closed"}\x1f${target_type}\x1f${selected_agent_id}`,
  );
  const [rooms_state, set_rooms_state] = useResettableState<ResourceState<RoomAggregate>>(
    { error: null, items: [], loading: should_load_rooms },
    `${is_open ? "open" : "closed"}\x1f${target_type}`,
  );
  const [room_contexts_state, set_room_contexts_state] = useResettableState<ResourceState<RoomContextAggregate>>(
    { error: null, items: [], loading: should_load_room_contexts },
    `${is_open ? "open" : "closed"}\x1f${target_type}\x1f${selected_room_id}`,
  );
  const { error: agents_error, items: agents, loading: agents_loading } = agents_state;
  const {
    error: agent_sessions_error,
    items: agent_sessions,
    loading: agent_sessions_loading,
  } = agent_sessions_state;
  const { error: rooms_error, items: rooms, loading: rooms_loading } = rooms_state;
  const {
    error: room_contexts_error,
    items: room_contexts,
    loading: room_contexts_loading,
  } = room_contexts_state;

  useEffect(() => {
    if (!should_load_agents) {
      return;
    }
    let cancelled = false;
    void get_agents()
      .then((next_agents) => {
        if (!cancelled) {
          set_agents_state({ error: null, items: next_agents, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_agents_state({
            error: error instanceof Error ? error.message : "加载智能体失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [set_agents_state, should_load_agents]);

  useEffect(() => {
    if (!should_load_rooms) {
      return;
    }
    let cancelled = false;
    void list_rooms(200)
      .then((next_rooms) => {
        if (!cancelled) {
          set_rooms_state({ error: null, items: next_rooms, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_rooms_state({
            error: error instanceof Error ? error.message : "加载 Room 列表失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [set_rooms_state, should_load_rooms]);

  useEffect(() => {
    if (!should_load_agent_sessions) return;
    let cancelled = false;
    void get_agent_sessions_api(selected_agent_id)
      .then((next_sessions) => {
        if (!cancelled) {
          set_agent_sessions_state({ error: null, items: next_sessions, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_agent_sessions_state({
            error: error instanceof Error ? error.message : "加载智能体会话失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selected_agent_id, set_agent_sessions_state, should_load_agent_sessions]);

  useEffect(() => {
    if (!should_load_room_contexts) return;
    let cancelled = false;
    void get_room_contexts(selected_room_id)
      .then((next_contexts) => {
        if (!cancelled) {
          set_room_contexts_state({ error: null, items: next_contexts, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_room_contexts_state({
            error: error instanceof Error ? error.message : "加载 Room 会话失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selected_room_id, set_room_contexts_state, should_load_room_contexts]);

  const agent_name_by_id = useMemo(
    () => new Map(agents.map((agent) => [agent.agent_id, agent.name])),
    [agents],
  );

  const agent_options = useMemo<ScheduledTaskDialogLabelOption[]>(
    () => agents.map((agent) => ({ value: agent.agent_id, label: agent.name || agent.agent_id })),
    [agents],
  );

  const room_options = useMemo<ScheduledTaskDialogLabelOption[]>(
    () => rooms.map((room) => ({ value: room.room.id, label: room.room.name?.trim() || room.room.id })),
    [rooms],
  );

  const agent_session_options = useMemo<ScheduledTaskDialogSessionOption[]>(
    () => agent_sessions.map((session) => ({
      value: session.session_key,
      session_key: session.session_key,
      agent_id: session.agent_id,
      label: format_session_label(session.title?.trim() || "未命名会话", agent_name_by_id.get(session.agent_id) || session.agent_id),
    })),
    [agent_name_by_id, agent_sessions],
  );

  const room_session_options = useMemo<ScheduledTaskDialogSessionOption[]>(() => {
    const options = build_room_session_selections(room_contexts, agent_name_by_id);
    return options.map((option) => ({
      value: option.value,
      session_key: option.session_key,
      agent_id: option.agent_id,
      label: option.label,
    }));
  }, [agent_name_by_id, room_contexts]);

  const session_options = target_type === "agent" ? agent_session_options : room_session_options;

  return {
    agents_loading,
    agent_sessions_loading,
    rooms_loading,
    room_contexts_loading,
    agents_error,
    agent_sessions_error,
    rooms_error,
    room_contexts_error,
    agent_options,
    room_options,
    session_options,
  };
}
