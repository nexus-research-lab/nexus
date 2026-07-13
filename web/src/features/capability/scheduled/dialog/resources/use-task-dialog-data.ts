"use client";

import { useMemo } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import { getAgentSessionsApi } from "@/lib/api/conversation/session-api";
import {
  getRoomContexts,
  listRooms,
} from "@/lib/api/conversation/room-resource-api";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type {
  RoomAggregate,
  RoomContextAggregate,
} from "@/types/conversation/room";

import type {
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  buildAgentNameIndex,
  buildAgentOptions,
  buildRoomOptions,
  buildTaskDialogResourceKeys,
  buildTaskDialogSessionData,
  resourceStatus,
} from "./task-dialog-resource-model";
import {
  type DialogResourceStatus,
  useDialogResource,
} from "./use-dialog-resource";

async function loadAgents(): Promise<Agent[]> {
  return getAgents();
}

async function loadRooms(): Promise<RoomAggregate[]> {
  return listRooms(200);
}

async function loadAgentSessions(agentId: string): Promise<AgentSession[]> {
  return getAgentSessionsApi(agentId);
}

async function loadRoomContexts(
  roomId: string,
): Promise<RoomContextAggregate[]> {
  return getRoomContexts(roomId);
}

export interface TaskDialogData {
  agentOptions: TaskDialogLabelOption[];
  agents: DialogResourceStatus;
  roomOptions: TaskDialogLabelOption[];
  rooms: DialogResourceStatus;
  sessionOptions: TaskDialogSessionOption[];
  sessions: DialogResourceStatus;
}

export function useTaskDialogData({
  form,
  isOpen,
}: {
  form: TaskFormDraft;
  isOpen: boolean;
}): TaskDialogData {
  const keys = buildTaskDialogResourceKeys(form, isOpen);
  const agents = useDialogResource(keys.agents, loadAgents, "加载智能体失败");
  const rooms = useDialogResource(keys.rooms, loadRooms, "加载 Room 列表失败");
  const agentSessions = useDialogResource(
    keys.agentSessions,
    loadAgentSessions,
    "加载智能体会话失败",
  );
  const roomContexts = useDialogResource(
    keys.roomContexts,
    loadRoomContexts,
    "加载 Room 会话失败",
  );
  const agentNameById = useMemo(
    () => buildAgentNameIndex(agents.items),
    [agents.items],
  );
  const agentOptions = useMemo(
    () => buildAgentOptions(agents.items),
    [agents.items],
  );
  const roomOptions = useMemo(
    () => buildRoomOptions(rooms.items),
    [rooms.items],
  );
  const sessionData = useMemo(
    () => buildTaskDialogSessionData(
      form.targetType,
      { agentSessions, roomContexts },
      agentNameById,
    ),
    [agentNameById, agentSessions, form.targetType, roomContexts],
  );

  return {
    agentOptions,
    agents: resourceStatus(agents),
    roomOptions,
    rooms: resourceStatus(rooms),
    sessionOptions: sessionData.options,
    sessions: sessionData.status,
  };
}
