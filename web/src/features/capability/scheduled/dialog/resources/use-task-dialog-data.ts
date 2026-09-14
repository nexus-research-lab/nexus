// INPUT: Automation 草稿、打开状态与当前语言。
// OUTPUT: 精确资源请求、Room 名称/紧凑会话候选与可恢复读取状态；不维护重复父名称索引。
// POS: 资源请求装配；选择身份留在草稿，公共名称规则不决定候选资格。
"use client";

import { useMemo } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import { buildAgentSelectionOptions } from "@/lib/agent-selection-options";
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
  TaskDestinationOption,
} from "../scheduled-task-dialog-types";
import {
  resolveTaskInheritedPermission,
  buildTaskDestinations,
  buildExecutionRoomOptions,
  buildExecutionRoomAgentData,
  buildDeliveryRoomAgentData,
  buildRoomOptions,
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
  inheritedPermissionMode: string | null;
  destinations: TaskDestinationOption[];
  destinationStatus: DialogResourceStatus;
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
  const agentOptions = useMemo(
    () => buildAgentSelectionOptions(agents.items, t),
    [agents.items, t],
  );
  const roomOptions = useMemo(
    () => buildExecutionRoomOptions(rooms.items, t),
    [rooms.items, t],
  );
  const deliveryRoomOptions = useMemo(
    () => buildRoomOptions(rooms.items, t),
    [rooms.items, t],
  );
  const sessionData = useMemo(
    () => buildTaskDialogSessionData(
      form.targetType,
      { agentSessions, roomContexts },
      t,
    ),
    [agentSessions, form.targetType, roomContexts, t],
  );
  const deliverySessionData = useMemo(
    () => buildTaskDialogDeliverySessionData(
      form,
      allSessions,
      t,
    ),
    [allSessions, form, t],
  );
  const executionRoomAgentData = useMemo(
    () => buildExecutionRoomAgentData(
      roomContexts.items,
      form.selectedSessionKey,
      agents.items,
      t,
    ),
    [agents.items, form.selectedSessionKey, roomContexts.items, t],
  );
  const deliveryRoomAgentData = useMemo(
    () => buildDeliveryRoomAgentData(
      allSessions.items,
      rooms.items,
      form.selectedDeliveryRoomId,
      form.selectedReplySessionKey,
      agents.items,
      t,
    ),
    [
      agents.items,
      allSessions.items,
      form.selectedDeliveryRoomId,
      form.selectedReplySessionKey,
      rooms.items,
      t,
    ],
  );
  const resolvedExecutionRoomId = form.targetType === "room"
    ? resolveTaskDialogRoomId(allSessions.items, form.selectedSessionKey)
    : "";
  const resolvedDeliveryRoomId = form.deliveryTargetType === "room"
    ? resolveTaskDialogRoomId(allSessions.items, form.selectedReplySessionKey)
    : "";
  return {
    inheritedPermissionMode: resolveTaskInheritedPermission(form, agents, agentSessions, roomContexts, executionRoomAgentData.defaultAgentId),
    destinations: buildTaskDestinations(allSessions.items, agentOptions, deliveryRoomOptions, t),
    destinationStatus: resourceStatus(allSessions),
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
