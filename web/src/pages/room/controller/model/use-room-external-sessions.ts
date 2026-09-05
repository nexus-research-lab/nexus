/**
 * INPUT: 当前 DM 的 exact Room + Agent scope 与 Agent Session 目录。
 * OUTPUT: 同 scope 的外部 Session 标签快照，以及 stale/error/显式刷新状态。
 * POS: Room 外部 Session 读取可靠性边界；普通失败保留旧快照，scope 或访问权限失效清空。
 */
"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { getAgentSessionsApi } from "@/lib/api/conversation/session-api";
import {
  getResourceFailure,
  type ResourceFailure,
} from "@/lib/error-message";
import { subscribeRoomDirectoryUpdates } from "@/lib/conversation/room-directory-events";
import {
  buildExternalSessionConversationId,
  formatExternalSessionTitle,
  isExternalSessionChannel,
} from "@/lib/conversation/external-session";
import { AgentSession } from "@/types/agent/agent";
import { RoomConversationView } from "@/types/conversation/conversation";

function buildExternalRoomConversationViews({
  roomId,
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
        external_identity: session.external_identity,
        external_session: true,
      },
      created_at: session.created_at,
      last_activity_at: session.last_activity_at,
      is_active: session.status === "active",
      is_draft: false,
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
      item.chat_type === other.chat_type &&
      JSON.stringify(item.external_identity) === JSON.stringify(other.external_identity);
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
  agentId,
  roomId,
  roomType,
}: {
  agentId: string | null;
  roomId: string | null;
  roomType: string | null;
}) {
  const externalSessionsResetKey = roomType === "dm" && agentId && roomId
    ? `${roomId}\u0000${agentId}`
    : "inactive";
  const [externalAgentSessions, setExternalAgentSessions] = useResettableState<AgentSession[]>(
    [],
    externalSessionsResetKey,
  );
  const [hasLoadedExternalSessions, setHasLoadedExternalSessions] = useResettableState(
    false,
    externalSessionsResetKey,
  );
  const [hasSuccessfulExternalSessionSnapshot, setHasSuccessfulExternalSessionSnapshot] =
    useResettableState(false, externalSessionsResetKey);
  const [externalSessionFailure, setExternalSessionFailure] =
    useResettableState<ResourceFailure | null>(null, externalSessionsResetKey);
  const [isRefreshingExternalSessions, setIsRefreshingExternalSessions] =
    useResettableState(false, externalSessionsResetKey);
  const [externalSessionRefreshVersion, setExternalSessionRefreshVersion] = useState(0);
  const refreshExternalSessions = useCallback(() => {
    setExternalSessionRefreshVersion((version) => version + 1);
  }, []);

  useEffect(
    () => subscribeRoomDirectoryUpdates(() => {
      setExternalSessionRefreshVersion((version) => version + 1);
    }),
    [],
  );

  useEffect(() => {
    if (roomType !== "dm" || !agentId || !roomId) {
      return undefined;
    }

    let cancelled = false;
    let refreshRequestId = 0;
    const refreshExternalSessions = () => {
      refreshRequestId += 1;
      const requestId = refreshRequestId;
      setIsRefreshingExternalSessions(true);
      void getAgentSessionsApi(agentId)
        .then((sessions) => {
          if (cancelled || requestId !== refreshRequestId) {
            return;
          }
          const nextSessions = filterExternalAgentSessions(sessions);
          setExternalAgentSessions((currentSessions) => (
            areExternalAgentSessionsEqual(currentSessions, nextSessions)
              ? currentSessions
              : nextSessions
          ));
          setExternalSessionFailure(null);
          setHasLoadedExternalSessions(true);
          setHasSuccessfulExternalSessionSnapshot(true);
          setIsRefreshingExternalSessions(false);
        })
        .catch((error) => {
          if (!cancelled && requestId === refreshRequestId) {
            console.error("[RoomPage] 加载 Agent 外部 IM 会话失败:", error);
            const failure = getResourceFailure(
              error,
              "外部会话标签暂时无法更新",
            );
            if (failure.access) {
              setExternalAgentSessions([]);
              setHasSuccessfulExternalSessionSnapshot(false);
            }
            setExternalSessionFailure(failure);
            setHasLoadedExternalSessions(true);
            setIsRefreshingExternalSessions(false);
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
    window.addEventListener("focus", refreshIfVisible);
    document.addEventListener("visibilitychange", refreshIfVisible);

    return () => {
      cancelled = true;
      window.removeEventListener("focus", refreshIfVisible);
      document.removeEventListener("visibilitychange", refreshIfVisible);
    };
  }, [
    agentId,
    externalSessionsResetKey,
    externalSessionRefreshVersion,
    roomId,
    roomType,
    setExternalAgentSessions,
    setExternalSessionFailure,
    setHasLoadedExternalSessions,
    setHasSuccessfulExternalSessionSnapshot,
    setIsRefreshingExternalSessions,
  ]);

  const externalRoomConversations = useMemo(
    () => buildExternalRoomConversationViews({
      roomId,
      sessions: externalAgentSessions,
    }),
    [externalAgentSessions, roomId],
  );

  return {
    externalAgentSessions,
    externalRoomConversations,
    externalSessionFailure,
    isExternalSessionCatalogStale: Boolean(
      externalSessionFailure && hasSuccessfulExternalSessionSnapshot,
    ),
    isRefreshingExternalSessions,
    isExternalSessionCatalogReady:
      roomType !== "dm" || !agentId || !roomId || hasLoadedExternalSessions,
    refreshExternalSessions,
  };
}
