"use client";

import { useEffect, useMemo, useState } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { get_agent_sessions_api } from "@/lib/api/agent-api";
import { subscribe_room_directory_updates } from "@/lib/api/room-api";
import {
  build_external_session_conversation_id,
  format_external_session_title,
  is_external_session_channel,
} from "@/features/conversation/external-session-labels";
import { AgentSession } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";

const EXTERNAL_AGENT_SESSION_FALLBACK_REFRESH_INTERVAL_MS = 60000;

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

export function useRoomExternalSessions({
  agent_id,
  room_id,
  room_type,
}: {
  agent_id: string | null;
  room_id: string | null;
  room_type: string | null;
}) {
  const external_sessions_reset_key = room_type === "dm" && agent_id ? agent_id : "inactive";
  const [external_agent_sessions, set_external_agent_sessions] = useResettableState<AgentSession[]>(
    [],
    external_sessions_reset_key,
  );
  const [external_session_refresh_version, set_external_session_refresh_version] = useState(0);

  useEffect(
    () => subscribe_room_directory_updates(() => {
      set_external_session_refresh_version((version) => version + 1);
    }),
    [],
  );

  useEffect(() => {
    if (room_type !== "dm" || !agent_id) {
      return undefined;
    }

    let cancelled = false;
    const refresh_external_sessions = () => {
      void get_agent_sessions_api(agent_id)
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
    const interval_id = window.setInterval(refresh_if_visible, EXTERNAL_AGENT_SESSION_FALLBACK_REFRESH_INTERVAL_MS);
    window.addEventListener("focus", refresh_if_visible);
    document.addEventListener("visibilitychange", refresh_if_visible);

    return () => {
      cancelled = true;
      window.clearInterval(interval_id);
      window.removeEventListener("focus", refresh_if_visible);
      document.removeEventListener("visibilitychange", refresh_if_visible);
    };
  }, [agent_id, external_session_refresh_version, room_type]);

  const external_room_conversations = useMemo(
    () => build_external_room_conversation_views({
      room_id,
      sessions: external_agent_sessions,
    }),
    [external_agent_sessions, room_id],
  );

  return {
    external_agent_sessions,
    external_room_conversations,
  };
}
