"use client";

import { useEffect, useMemo } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { getAgents } from "@/lib/api/agent-manage-api";
import { getAgentSessionsApi } from "@/lib/api/agent-api";
import { getRoomContexts, listRooms } from "@/lib/api/room-api";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";

import {
  buildRoomSessionSelections,
  formatSessionLabel,
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
  isOpen,
  targetType,
  selectedAgentId,
  selectedRoomId,
}: {
  isOpen: boolean;
  targetType: TargetType;
  selectedAgentId: string;
  selectedRoomId: string;
}) {
  const shouldLoadAgents = isOpen;
  const shouldLoadRooms = isOpen && targetType === "room";
  const shouldLoadAgentSessions = isOpen && targetType === "agent" && Boolean(selectedAgentId);
  const shouldLoadRoomContexts = isOpen && targetType === "room" && Boolean(selectedRoomId);
  const [agentsState, setAgentsState] = useResettableState<ResourceState<Agent>>(
    { error: null, items: [], loading: shouldLoadAgents },
    isOpen ? "open" : "closed",
  );
  const [agentSessionsState, setAgentSessionsState] = useResettableState<ResourceState<AgentSession>>(
    { error: null, items: [], loading: shouldLoadAgentSessions },
    `${isOpen ? "open" : "closed"}\x1f${targetType}\x1f${selectedAgentId}`,
  );
  const [roomsState, setRoomsState] = useResettableState<ResourceState<RoomAggregate>>(
    { error: null, items: [], loading: shouldLoadRooms },
    `${isOpen ? "open" : "closed"}\x1f${targetType}`,
  );
  const [roomContextsState, setRoomContextsState] = useResettableState<ResourceState<RoomContextAggregate>>(
    { error: null, items: [], loading: shouldLoadRoomContexts },
    `${isOpen ? "open" : "closed"}\x1f${targetType}\x1f${selectedRoomId}`,
  );
  const { error: agentsError, items: agents, loading: agentsLoading } = agentsState;
  const {
    error: agentSessionsError,
    items: agentSessions,
    loading: agentSessionsLoading,
  } = agentSessionsState;
  const { error: roomsError, items: rooms, loading: roomsLoading } = roomsState;
  const {
    error: roomContextsError,
    items: roomContexts,
    loading: roomContextsLoading,
  } = roomContextsState;

  useEffect(() => {
    if (!shouldLoadAgents) {
      return;
    }
    let cancelled = false;
    void getAgents()
      .then((nextAgents) => {
        if (!cancelled) {
          setAgentsState({ error: null, items: nextAgents, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setAgentsState({
            error: error instanceof Error ? error.message : "加载智能体失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [setAgentsState, shouldLoadAgents]);

  useEffect(() => {
    if (!shouldLoadRooms) {
      return;
    }
    let cancelled = false;
    void listRooms(200)
      .then((nextRooms) => {
        if (!cancelled) {
          setRoomsState({ error: null, items: nextRooms, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setRoomsState({
            error: error instanceof Error ? error.message : "加载 Room 列表失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [setRoomsState, shouldLoadRooms]);

  useEffect(() => {
    if (!shouldLoadAgentSessions) return;
    let cancelled = false;
    void getAgentSessionsApi(selectedAgentId)
      .then((nextSessions) => {
        if (!cancelled) {
          setAgentSessionsState({ error: null, items: nextSessions, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setAgentSessionsState({
            error: error instanceof Error ? error.message : "加载智能体会话失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedAgentId, setAgentSessionsState, shouldLoadAgentSessions]);

  useEffect(() => {
    if (!shouldLoadRoomContexts) return;
    let cancelled = false;
    void getRoomContexts(selectedRoomId)
      .then((nextContexts) => {
        if (!cancelled) {
          setRoomContextsState({ error: null, items: nextContexts, loading: false });
        }
      })
      .catch((error) => {
        if (!cancelled) {
          setRoomContextsState({
            error: error instanceof Error ? error.message : "加载 Room 会话失败",
            items: [],
            loading: false,
          });
        }
      });
    return () => {
      cancelled = true;
    };
  }, [selectedRoomId, setRoomContextsState, shouldLoadRoomContexts]);

  const agentNameById = useMemo(
    () => new Map(agents.map((agent) => [agent.agent_id, agent.name])),
    [agents],
  );

  const agentOptions = useMemo<ScheduledTaskDialogLabelOption[]>(
    () => agents.map((agent) => ({ value: agent.agent_id, label: agent.name || agent.agent_id })),
    [agents],
  );

  const roomOptions = useMemo<ScheduledTaskDialogLabelOption[]>(
    () => rooms.map((room) => ({ value: room.room.id, label: room.room.name?.trim() || room.room.id })),
    [rooms],
  );

  const agentSessionOptions = useMemo<ScheduledTaskDialogSessionOption[]>(
    () => agentSessions.map((session) => ({
      value: session.session_key,
      sessionKey: session.session_key,
      agentId: session.agent_id,
      label: formatSessionLabel(session.title?.trim() || "未命名会话", agentNameById.get(session.agent_id) || session.agent_id),
    })),
    [agentNameById, agentSessions],
  );

  const roomSessionOptions = useMemo<ScheduledTaskDialogSessionOption[]>(() => {
    const options = buildRoomSessionSelections(roomContexts, agentNameById);
    return options.map((option) => ({
      value: option.value,
      sessionKey: option.session_key,
      agentId: option.agent_id,
      label: option.label,
    }));
  }, [agentNameById, roomContexts]);

  const sessionOptions = targetType === "agent" ? agentSessionOptions : roomSessionOptions;

  return {
    agentsLoading: agentsLoading,
    agentSessionsLoading: agentSessionsLoading,
    roomsLoading: roomsLoading,
    roomContextsLoading: roomContextsLoading,
    agentsError: agentsError,
    agentSessionsError: agentSessionsError,
    roomsError: roomsError,
    roomContextsError: roomContextsError,
    agentOptions: agentOptions,
    roomOptions: roomOptions,
    sessionOptions: sessionOptions,
  };
}
