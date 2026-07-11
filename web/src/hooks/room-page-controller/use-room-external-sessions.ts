"use client";

import { useEffect, useMemo, useState } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getAgentSessionsApi } from "@/lib/api/agent-api";
import { subscribeRoomDirectoryUpdates } from "@/lib/api/room-api";
import {
  buildExternalSessionConversationId,
  formatExternalSessionTitle,
  isExternalSessionChannel,
} from "@/lib/conversation/external-session";
import { AgentSession } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";

const EXTERNAL_AGENT_SESSION_FALLBACK_REFRESH_INTERVAL_MS = 60000;

function buildExternalRoomConversationViews({
  roomId: roomId,
  sessions,
}: {
  roomId: string | null;
  sessions: AgentSession[];
}): RoomConversationView[] {
  if (!roomId) {
    return [];
  }
  return sessions
    .filter((session) => (
      !session.room_id &&
      isExternalSessionChannel(session.channel_type, session.session_key)
    ))
    .map((session) => ({
      session_key: session.session_key,
      room_id: roomId,
      conversation_id: buildExternalSessionConversationId(session.session_key),
      conversation_type: "external",
      session_id: session.session_id,
      agent_id: session.agent_id,
      title: formatExternalSessionTitle({
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

function areExternalAgentSessionsEqual(left: AgentSession[], right: AgentSession[]): boolean {
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

function filterExternalAgentSessions(sessions: AgentSession[]): AgentSession[] {
  return sessions
    .filter((item) => (
      !item.room_id &&
      isExternalSessionChannel(item.channel_type, item.session_key)
    ))
    .sort((left, right) => right.last_activity_at - left.last_activity_at);
}

export function useRoomExternalSessions({
  agentId: agentId,
  roomId: roomId,
  roomType: roomType,
}: {
  agentId: string | null;
  roomId: string | null;
  roomType: string | null;
}) {
  const externalSessionsResetKey = roomType === "dm" && agentId ? agentId : "inactive";
  const [externalAgentSessions, setExternalAgentSessions] = useResettableState<AgentSession[]>(
    [],
    externalSessionsResetKey,
  );
  const [externalSessionRefreshVersion, setExternalSessionRefreshVersion] = useState(0);

  useEffect(
    () => subscribeRoomDirectoryUpdates(() => {
      setExternalSessionRefreshVersion((version) => version + 1);
    }),
    [],
  );

  useEffect(() => {
    if (roomType !== "dm" || !agentId) {
      return undefined;
    }

    let cancelled = false;
    const refreshExternalSessions = () => {
      void getAgentSessionsApi(agentId)
        .then((sessions) => {
          if (cancelled) {
            return;
          }
          const nextSessions = filterExternalAgentSessions(sessions);
          setExternalAgentSessions((currentSessions) => (
            areExternalAgentSessionsEqual(currentSessions, nextSessions)
              ? currentSessions
              : nextSessions
          ));
        })
        .catch((error) => {
          console.error("[RoomPage] 加载 Agent 外部 IM 会话失败:", error);
          if (!cancelled) {
            setExternalAgentSessions([]);
          }
        });
    };
    const refreshIfVisible = () => {
      if (cancelled) {
        return;
      }
      if (typeof document !== "undefined" && document.visibilityState === "hidden") {
        return;
      }
      refreshExternalSessions();
    };

    refreshExternalSessions();
    const intervalId = window.setInterval(refreshIfVisible, EXTERNAL_AGENT_SESSION_FALLBACK_REFRESH_INTERVAL_MS);
    window.addEventListener("focus", refreshIfVisible);
    document.addEventListener("visibilitychange", refreshIfVisible);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
      window.removeEventListener("focus", refreshIfVisible);
      document.removeEventListener("visibilitychange", refreshIfVisible);
    };
  }, [
    agentId,
    externalSessionRefreshVersion,
    roomType,
    setExternalAgentSessions,
  ]);

  const externalRoomConversations = useMemo(
    () => buildExternalRoomConversationViews({
      roomId: roomId,
      sessions: externalAgentSessions,
    }),
    [externalAgentSessions, roomId],
  );

  return {
    externalAgentSessions: externalAgentSessions,
    externalRoomConversations: externalRoomConversations,
  };
}
