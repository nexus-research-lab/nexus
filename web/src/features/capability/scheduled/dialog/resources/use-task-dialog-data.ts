"use client";

import { useMemo } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import { getAgentSessionsApi } from "@/lib/api/conversation/session-api";
import {
  getRoomContexts,
  listRooms,
} from "@/lib/api/conversation/room-resource-api";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  const { t } = useI18n();
  const keys = buildTaskDialogResourceKeys(form, isOpen);
  const agents = useDialogResource(
    keys.agents,
    loadAgents,
    t("capability.scheduled_dialog_load_agents_failed"),
  );
  const rooms = useDialogResource(
    keys.rooms,
    loadRooms,
    t("capability.scheduled_dialog_load_rooms_failed"),
  );
  const agentSessions = useDialogResource(
    keys.agentSessions,
    loadAgentSessions,
    t("capability.scheduled_dialog_load_agent_sessions_failed"),
  );
  const roomContexts = useDialogResource(
    keys.roomContexts,
    loadRoomContexts,
    t("capability.scheduled_dialog_load_room_sessions_failed"),
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
      t("capability.scheduled_dialog_unnamed_session"),
    ),
    [agentNameById, agentSessions, form.targetType, roomContexts, t],
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
