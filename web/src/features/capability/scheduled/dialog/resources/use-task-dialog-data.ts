"use client";

import { useMemo } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import {
  getAgentSessionsApi,
  getAllSessionsApi,
} from "@/lib/api/conversation/session-api";
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
  buildExecutionRoomOptions,
  buildExecutionRoomAgentData,
  buildDeliveryRoomAgentData,
  buildRoomOptions,
  buildRoomNameIndex,
  buildTaskDialogDeliverySessionData,
  buildTaskDialogResourceKeys,
  buildTaskDialogSessionData,
  resolveTaskDialogRoomId,
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

async function loadAllSessions(): Promise<AgentSession[]> {
  return getAllSessionsApi();
}

async function loadRoomContexts(
  roomId: string,
): Promise<RoomContextAggregate[]> {
  return getRoomContexts(roomId);
}

export interface TaskDialogData {
  agentOptions: TaskDialogLabelOption[];
  agents: DialogResourceStatus;
  deliveryRoomOptions: TaskDialogLabelOption[];
  deliveryRoomAgentOptions: TaskDialogLabelOption[];
  defaultDeliveryRoomAgentId: string;
  defaultExecutionRoomAgentId: string;
  executionRoomAgentOptions: TaskDialogLabelOption[];
  roomOptions: TaskDialogLabelOption[];
  rooms: DialogResourceStatus;
  deliverySessionOptions: TaskDialogSessionOption[];
  deliverySessions: DialogResourceStatus;
  resolvedDeliveryRoomId: string;
  resolvedExecutionRoomId: string;
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
  const allSessions = useDialogResource(
    keys.allSessions,
    loadAllSessions,
    t("capability.scheduled_dialog_load_agent_sessions_failed"),
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
    () => buildExecutionRoomOptions(rooms.items),
    [rooms.items],
  );
  const deliveryRoomOptions = useMemo(
    () => buildRoomOptions(rooms.items),
    [rooms.items],
  );
  const roomNameById = useMemo(
    () => buildRoomNameIndex(rooms.items),
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
  const deliverySessionData = useMemo(
    () => buildTaskDialogDeliverySessionData(
      form,
      allSessions,
      agentNameById,
      roomNameById,
      t("capability.scheduled_dialog_unnamed_session"),
    ),
    [agentNameById, allSessions, form, roomNameById, t],
  );
  const executionRoomAgentData = useMemo(
    () => buildExecutionRoomAgentData(
      roomContexts.items,
      form.selectedSessionKey,
    ),
    [form.selectedSessionKey, roomContexts.items],
  );
  const deliveryRoomAgentData = useMemo(
    () => buildDeliveryRoomAgentData(
      allSessions.items,
      rooms.items,
      form.selectedDeliveryRoomId,
      form.selectedReplySessionKey,
      agentNameById,
    ),
    [
      agentNameById,
      allSessions.items,
      form.selectedDeliveryRoomId,
      form.selectedReplySessionKey,
      rooms.items,
    ],
  );
  const resolvedExecutionRoomId = form.targetType === "room"
    ? resolveTaskDialogRoomId(allSessions.items, form.selectedSessionKey)
    : "";
  const resolvedDeliveryRoomId = form.deliveryTargetType === "room"
    ? resolveTaskDialogRoomId(allSessions.items, form.selectedReplySessionKey)
    : "";
  return {
    agentOptions,
    agents: resourceStatus(agents),
    deliveryRoomOptions,
    deliveryRoomAgentOptions: deliveryRoomAgentData.options,
    defaultDeliveryRoomAgentId: deliveryRoomAgentData.defaultAgentId,
    defaultExecutionRoomAgentId: executionRoomAgentData.defaultAgentId,
    executionRoomAgentOptions: executionRoomAgentData.options,
    roomOptions,
    rooms: resourceStatus(rooms),
    deliverySessionOptions: deliverySessionData.options,
    deliverySessions: deliverySessionData.status,
    resolvedDeliveryRoomId,
    resolvedExecutionRoomId,
    sessionOptions: sessionData.options,
    sessions: sessionData.status,
  };
}
