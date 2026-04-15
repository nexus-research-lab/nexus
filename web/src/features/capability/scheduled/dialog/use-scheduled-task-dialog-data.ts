"use client";

import { useEffect, useMemo, useState } from "react";

import { get_agents } from "@/lib/agent-manage-api";
import { get_agent_sessions_api } from "@/lib/agent-api";
import { get_room_contexts, list_rooms } from "@/lib/room-api";
import type { Agent, AgentSession } from "@/types/agent";
import type { RoomAggregate, RoomContextAggregate } from "@/types/room";

import {
  build_room_session_selections,
  format_session_label,
  type TargetType,
} from "./scheduled-task-dialog-constants";

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
  const [agents, set_agents] = useState<Agent[]>([]);
  const [agent_sessions, set_agent_sessions] = useState<AgentSession[]>([]);
  const [rooms, set_rooms] = useState<RoomAggregate[]>([]);
  const [room_contexts, set_room_contexts] = useState<RoomContextAggregate[]>([]);
  const [agents_loading, set_agents_loading] = useState(false);
  const [agent_sessions_loading, set_agent_sessions_loading] = useState(false);
  const [rooms_loading, set_rooms_loading] = useState(false);
  const [room_contexts_loading, set_room_contexts_loading] = useState(false);
  const [agents_error, set_agents_error] = useState<string | null>(null);
  const [agent_sessions_error, set_agent_sessions_error] = useState<string | null>(null);
  const [rooms_error, set_rooms_error] = useState<string | null>(null);
  const [room_contexts_error, set_room_contexts_error] = useState<string | null>(null);

  useEffect(() => {
    if (!is_open) {
      return;
    }
    let cancelled = false;
    set_agents_loading(true);
    set_agents_error(null);
    void get_agents()
      .then((next_agents) => {
        if (!cancelled) {
          set_agents(next_agents);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_agents_error(error instanceof Error ? error.message : "加载智能体失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_agents_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open]);

  useEffect(() => {
    if (!is_open || target_type !== "room") {
      return;
    }
    let cancelled = false;
    set_rooms_loading(true);
    set_rooms_error(null);
    void list_rooms(200)
      .then((next_rooms) => {
        if (!cancelled) {
          set_rooms(next_rooms);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_rooms_error(error instanceof Error ? error.message : "加载 Room 列表失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_rooms_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open, target_type]);

  useEffect(() => {
    if (!is_open || target_type !== "agent" || !selected_agent_id) {
      set_agent_sessions([]);
      return;
    }
    let cancelled = false;
    set_agent_sessions_loading(true);
    set_agent_sessions_error(null);
    void get_agent_sessions_api(selected_agent_id)
      .then((next_sessions) => {
        if (!cancelled) {
          set_agent_sessions(next_sessions);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_agent_sessions_error(error instanceof Error ? error.message : "加载智能体会话失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_agent_sessions_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open, selected_agent_id, target_type]);

  useEffect(() => {
    if (!is_open || target_type !== "room" || !selected_room_id) {
      set_room_contexts([]);
      return;
    }
    let cancelled = false;
    set_room_contexts_loading(true);
    set_room_contexts_error(null);
    void get_room_contexts(selected_room_id)
      .then((next_contexts) => {
        if (!cancelled) {
          set_room_contexts(next_contexts);
        }
      })
      .catch((error) => {
        if (!cancelled) {
          set_room_contexts_error(error instanceof Error ? error.message : "加载 Room 会话失败");
        }
      })
      .finally(() => {
        if (!cancelled) {
          set_room_contexts_loading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [is_open, selected_room_id, target_type]);

  const agent_name_by_id = useMemo(
    () => new Map(agents.map((agent) => [agent.agent_id, agent.name])),
    [agents],
  );

  const agent_options = useMemo(
    () => agents.map((agent) => ({ value: agent.agent_id, label: agent.name || agent.agent_id })),
    [agents],
  );

  const room_options = useMemo(
    () => rooms.map((room) => ({ value: room.room.id, label: room.room.name?.trim() || room.room.id })),
    [rooms],
  );

  const agent_session_options = useMemo(
    () => agent_sessions.map((session) => ({
      session_key: session.session_key,
      agent_id: session.agent_id,
      label: format_session_label(session.title?.trim() || "未命名会话", agent_name_by_id.get(session.agent_id) || session.agent_id),
    })),
    [agent_name_by_id, agent_sessions],
  );

  const room_session_options = useMemo(() => {
    const options = build_room_session_selections(room_contexts, agent_name_by_id);
    return options.map((option) => ({
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
