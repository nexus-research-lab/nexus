"use client";

import { useMemo } from "react";

import { getAgentSessionsApi } from "@/lib/api/conversation/session-api";
import { getAgents } from "@/lib/api/agent/agent-api";
import { getRoomContexts, listRooms } from "@/lib/api/conversation/room-resource-api";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type { RoomAggregate, RoomContextAggregate } from "@/types/conversation/room";

import type {
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import {
  buildRoomSessionSelections,
  formatSessionLabel,
} from "../schedule/task-schedule-time";
import { type DialogResource, useDialogResource } from "./use-dialog-resource";

const OPEN_RESOURCE_KEY = "open";

async function loadAgents(): Promise<Agent[]> {
  return getAgents();
}

async function loadRooms(): Promise<RoomAggregate[]> {
  return listRooms(200);
}

async function loadAgentSessions(agentId: string): Promise<AgentSession[]> {
  return getAgentSessionsApi(agentId);
}

async function loadRoomContexts(roomId: string): Promise<RoomContextAggregate[]> {
  return getRoomContexts(roomId);
}

function buildAgentOptions(agents: Agent[]): TaskDialogLabelOption[] {
  return agents.map((agent) => ({
    label: agent.name || agent.agent_id,
    value: agent.agent_id,
  }));
}

function buildRoomOptions(rooms: RoomAggregate[]): TaskDialogLabelOption[] {
  return rooms.map((room) => ({
    label: room.room.name?.trim() || room.room.id,
    value: room.room.id,
  }));
}

function buildAgentSessionOptions(
  sessions: AgentSession[],
  agentNameById: Map<string, string>,
): TaskDialogSessionOption[] {
  return sessions.map((session) => ({
    agentId: session.agent_id,
    label: formatSessionLabel(
      session.title?.trim() || "未命名会话",
      agentNameById.get(session.agent_id) || session.agent_id,
    ),
    sessionKey: session.session_key,
    value: session.session_key,
  }));
}

function buildRoomSessionOptions(
  contexts: RoomContextAggregate[],
  agentNameById: Map<string, string>,
): TaskDialogSessionOption[] {
  return buildRoomSessionSelections(contexts, agentNameById).map((option) => ({
    agentId: option.agent_id,
    label: option.label,
    sessionKey: option.session_key,
    value: option.value,
  }));
}

export interface TaskDialogData {
  agentOptions: TaskDialogLabelOption[];
  agents: DialogResource<Agent>;
  roomOptions: TaskDialogLabelOption[];
  rooms: DialogResource<RoomAggregate>;
  sessionOptions: TaskDialogSessionOption[];
  sessions: DialogResource<AgentSession | RoomContextAggregate>;
}

export function useTaskDialogData({
  form,
  isOpen,
}: {
  form: TaskFormDraft;
  isOpen: boolean;
}): TaskDialogData {
  const needsAgentSessions = form.executionKind === "agent"
    && form.targetType === "agent"
    && (form.executionMode === "existing" || form.replyMode === "selected");
  const needsRoomContexts = form.executionKind === "agent"
    && form.targetType === "room";

  const agents = useDialogResource(
    isOpen ? OPEN_RESOURCE_KEY : null,
    loadAgents,
    "加载智能体失败",
  );
  const rooms = useDialogResource(
    isOpen && form.targetType === "room" ? OPEN_RESOURCE_KEY : null,
    loadRooms,
    "加载 Room 列表失败",
  );
  const agentSessions = useDialogResource(
    isOpen && needsAgentSessions && form.selectedAgentId
      ? form.selectedAgentId
      : null,
    loadAgentSessions,
    "加载智能体会话失败",
  );
  const roomContexts = useDialogResource(
    isOpen && needsRoomContexts && form.selectedRoomId
      ? form.selectedRoomId
      : null,
    loadRoomContexts,
    "加载 Room 会话失败",
  );

  const agentNameById = useMemo(
    () => new Map(agents.items.map((agent) => [agent.agent_id, agent.name])),
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
  const agentSessionOptions = useMemo(
    () => buildAgentSessionOptions(agentSessions.items, agentNameById),
    [agentNameById, agentSessions.items],
  );
  const roomSessionOptions = useMemo(
    () => buildRoomSessionOptions(roomContexts.items, agentNameById),
    [agentNameById, roomContexts.items],
  );

  return {
    agentOptions,
    agents,
    roomOptions,
    rooms,
    sessionOptions: form.targetType === "room"
      ? roomSessionOptions
      : agentSessionOptions,
    sessions: form.targetType === "room" ? roomContexts : agentSessions,
  };
}
