/**
 * INPUT: Automation 表单目标、Agent/Room 目录与统一 Session 读模型。
 * OUTPUT: 严格按 DM/Room 身份隔离的执行与结果接收候选、资源状态和 Room 解析。
 * POS: 定时任务弹窗所有目标与 Session 候选的唯一纯投影入口。
 */
import {
  getExternalSessionChannelLabel,
  getExternalSessionDisplayLabel,
} from "@/lib/conversation/external-session";
import {
  buildRoomSharedSessionKey,
  parseSessionKey,
} from "@/lib/conversation/session-key";
import type { Agent, AgentSession } from "@/types/agent/agent";
import type {
  RoomAggregate,
  RoomContextAggregate,
} from "@/types/conversation/room";

import type {
  TargetType,
  TaskDialogLabelOption,
  TaskDialogSessionOption,
  TaskFormDraft,
} from "../scheduled-task-dialog-types";
import { formatSessionLabel } from "../schedule/task-schedule-time";
import type {
  DialogResource,
  DialogResourceStatus,
} from "./use-dialog-resource";

const OPEN_RESOURCE_KEY = "open";

export interface TaskDialogResourceKeys {
  allSessions: string | null;
  agentSessions: string | null;
  agents: string | null;
  roomContexts: string | null;
  rooms: string | null;
}

export interface TaskDialogSessionData {
  options: TaskDialogSessionOption[];
  status: DialogResourceStatus;
}

export interface TaskDialogRoomAgentData {
  defaultAgentId: string;
  options: TaskDialogLabelOption[];
}

interface TaskDialogSessionResources {
  agentSessions: DialogResource<AgentSession>;
  roomContexts: DialogResource<RoomContextAggregate>;
}

const SESSION_REQUEST_KEYS: Record<
  TargetType,
  (form: TaskFormDraft, isOpen: boolean) => Pick<
    TaskDialogResourceKeys,
    "agentSessions" | "roomContexts"
  >
> = {
  agent: (form, isOpen) => ({
    agentSessions: activeResourceKey(
      isOpen
        && form.executionKind === "agent"
        && form.executionMode === "existing",
      form.selectedAgentId,
    ),
    roomContexts: null,
  }),
  room: (form, isOpen) => ({
    agentSessions: null,
    roomContexts: activeResourceKey(
      isOpen && form.executionKind === "agent",
      form.selectedRoomId,
    ),
  }),
};

const SESSION_DATA_BUILDERS: Record<
  TargetType,
  (
    resources: TaskDialogSessionResources,
    agentNameById: Map<string, string>,
    unnamedSessionLabel: string,
  ) => TaskDialogSessionData
> = {
  agent: ({ agentSessions }, agentNameById, unnamedSessionLabel) => ({
    options: buildAgentSessionOptions(
      agentSessions.items,
      agentNameById,
      unnamedSessionLabel,
    ),
    status: resourceStatus(agentSessions),
  }),
  room: ({ roomContexts }, agentNameById, unnamedSessionLabel) => ({
    options: buildRoomSessionOptions(
      roomContexts.items,
      agentNameById,
      unnamedSessionLabel,
    ),
    status: resourceStatus(roomContexts),
  }),
};

export function buildTaskDialogResourceKeys(
  form: TaskFormDraft,
  isOpen: boolean,
): TaskDialogResourceKeys {
  return {
    allSessions: isOpen && form.executionKind === "agent" && (
      form.targetType === "room" || form.replyMode === "selected"
    )
      ? OPEN_RESOURCE_KEY
      : null,
    agents: isOpen ? OPEN_RESOURCE_KEY : null,
    rooms: isOpen && (
      form.targetType === "room"
      || (form.replyMode === "selected" && form.deliveryTargetType === "room")
    ) ? OPEN_RESOURCE_KEY : null,
    ...SESSION_REQUEST_KEYS[form.targetType](form, isOpen),
  };
}

export function buildAgentNameIndex(agents: Agent[]): Map<string, string> {
  return new Map(agents.map((agent) => [agent.agent_id, agent.name]));
}

export function buildAgentOptions(agents: Agent[]): TaskDialogLabelOption[] {
  return agents.map((agent) => ({
    label: agent.name || agent.agent_id,
    value: agent.agent_id,
  }));
}

export function buildRoomOptions(
  rooms: RoomAggregate[],
): TaskDialogLabelOption[] {
  return rooms.filter(isGroupRoom).map((room) => ({
    label: room.room.name?.trim() || room.room.id,
    value: room.room.id,
  }));
}

export function buildExecutionRoomOptions(
  rooms: RoomAggregate[],
): TaskDialogLabelOption[] {
  return rooms.filter((room) => (
    isGroupRoom(room)
    && room.members.some((member) => (
      member.member_type === "agent" && !member.participation_paused
    ))
  )).map((room) => ({
    label: room.room.name?.trim() || room.room.id,
    value: room.room.id,
  }));
}

export function buildTaskDialogSessionData(
  targetType: TargetType,
  resources: TaskDialogSessionResources,
  agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionData {
  return SESSION_DATA_BUILDERS[targetType](
    resources,
    agentNameById,
    unnamedSessionLabel,
  );
}

export function buildTaskDialogDeliverySessionData(
  form: TaskFormDraft,
  sessions: DialogResource<AgentSession>,
  agentNameById: Map<string, string>,
  roomNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionData {
  if (form.replyMode !== "selected") {
    return { options: [], status: resourceStatus(sessions) };
  }
  const options = form.deliveryTargetType === "room"
    ? buildDeliveryRoomOptions(
        sessions.items,
        form.selectedDeliveryRoomId,
        roomNameById,
        unnamedSessionLabel,
      )
    : buildDeliveryAgentOptions(
        sessions.items,
        form.selectedDeliveryAgentId,
        agentNameById,
        unnamedSessionLabel,
      );
  return { options, status: resourceStatus(sessions) };
}

export function buildExecutionRoomAgentData(
  contexts: RoomContextAggregate[],
  selectedSessionKey: string,
): TaskDialogRoomAgentData {
  const conversationId = roomConversationId(selectedSessionKey);
  const context = contexts.find((item) => (
    item.room.room_type === "room"
    && item.conversation.id === conversationId
  ));
  if (!context) {
    return { defaultAgentId: "", options: [] };
  }
  const availableAgentIds = new Set(context.sessions.map((session) => (
    session.agent_id.trim()
  )).filter(Boolean));
  const options = context.member_agents.filter((agent) => (
    availableAgentIds.has(agent.agent_id)
    && !agent.room_participation_paused
  )).map((agent) => ({
    label: agent.name || agent.agent_id,
    value: agent.agent_id,
  }));
  const defaultAgentId = context.room.host_agent_id?.trim() || "";
  return {
    defaultAgentId: options.some((option) => option.value === defaultAgentId)
      ? defaultAgentId
      : "",
    options,
  };
}

export function buildDeliveryRoomAgentData(
  sessions: AgentSession[],
  rooms: RoomAggregate[],
  roomId: string,
  selectedSessionKey: string,
  agentNameById: Map<string, string>,
): TaskDialogRoomAgentData {
  const normalizedRoomId = roomId.trim();
  const conversationId = roomConversationId(selectedSessionKey);
  if (!normalizedRoomId || !conversationId) {
    return { defaultAgentId: "", options: [] };
  }
  const seen = new Set<string>();
  const options: TaskDialogLabelOption[] = [];
  sessions.filter((session) => (
    session.room_id === normalizedRoomId
    && session.conversation_id === conversationId
    && isRoomMemberSession(session)
  )).forEach((session) => {
    const agentId = session.agent_id.trim();
    if (!agentId || seen.has(agentId)) {
      return;
    }
    seen.add(agentId);
    options.push({
      label: agentNameById.get(agentId) || agentId,
      value: agentId,
    });
  });
  const defaultAgentId = rooms.find((room) => (
    isGroupRoom(room) && room.room.id === normalizedRoomId
  ))?.room.host_agent_id?.trim() || "";
  return {
    defaultAgentId: options.some((option) => option.value === defaultAgentId)
      ? defaultAgentId
      : "",
    options,
  };
}

export function buildRoomNameIndex(
  rooms: RoomAggregate[],
): Map<string, string> {
  return new Map(rooms.map((room) => [
    room.room.id,
    room.room.name?.trim() || room.room.id,
  ]));
}

export function resolveTaskDialogRoomId(
  sessions: AgentSession[],
  sessionKey: string,
): string {
  const parsed = parseSessionKey(sessionKey.split("::executor:", 1)[0]);
  const conversationId = parsed.conversation_id || parsed.ref || "";
  if (!conversationId) {
    return "";
  }
  return sessions.find((session) => (
    isRoomMemberSession(session)
    &&
    session.conversation_id === conversationId
    && Boolean(session.room_id)
  ))?.room_id?.trim() || "";
}

function roomConversationId(sessionKey: string): string {
  const parsed = parseSessionKey(sessionKey);
  if (!parsed.is_structured || parsed.kind !== "room") {
    return "";
  }
  return parsed.conversation_id || parsed.ref || "";
}

export function resourceStatus<T>(
  resource: DialogResource<T>,
): DialogResourceStatus {
  return {
    error: resource.error,
    loading: resource.loading,
  };
}

function activeResourceKey(
  active: boolean,
  selectedId: string,
): string | null {
  return active && selectedId ? selectedId : null;
}

function isGroupRoom(room: RoomAggregate): boolean {
  return room.room.room_type.trim().toLowerCase() === "room";
}

function normalizeSessionChatType(value: string | null): string {
  switch (value?.trim().toLowerCase()) {
    case "":
    case undefined:
    case "dm":
      return "dm";
    case "group":
    case "room":
      return "group";
    default:
      return value?.trim().toLowerCase() || "";
  }
}

function isAgentSessionOfChatType(
  session: AgentSession,
  expectedChatType: "dm" | "group",
): boolean {
  const parsed = parseSessionKey(session.session_key);
  if (!parsed.is_structured
    || parsed.kind !== "agent"
    || normalizeSessionChatType(parsed.chat_type) !== expectedChatType) {
    return false;
  }
  const storedChatType = session.chat_type.trim();
  return !storedChatType
    || normalizeSessionChatType(storedChatType) === expectedChatType;
}

function isAgentDMSession(session: AgentSession): boolean {
  return isAgentSessionOfChatType(session, "dm");
}

function isRoomMemberSession(session: AgentSession): boolean {
  return isAgentSessionOfChatType(session, "group");
}

function buildAgentSessionOptions(
  sessions: AgentSession[],
  agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  return sessions.filter((session) => {
    if (!isAgentDMSession(session)) {
      return false;
    }
    const externalChannel = getExternalSessionChannelLabel(
      session.channel_type,
      session.session_key,
    );
    return !externalChannel || session.external_identity?.current_pairing === true;
  }).map((session) => {
    const channelLabel = getExternalSessionDisplayLabel(
      session.channel_type,
      session.session_key,
      session.external_identity,
    );
    return {
      badge: channelLabel ? `IM · ${channelLabel}` : null,
      label: formatSessionLabel(
        session.title?.trim() || unnamedSessionLabel,
        agentNameById.get(session.agent_id) || session.agent_id,
      ),
      sessionKey: session.session_key,
      value: session.session_key,
    };
  });
}

function isAvailableDeliverySession(session: AgentSession): boolean {
  const externalChannel = getExternalSessionChannelLabel(
    session.channel_type,
    session.session_key,
  );
  return !externalChannel || session.external_identity?.current_pairing === true;
}

function isUserVisibleDeliverySession(session: AgentSession): boolean {
  const parsed = parseSessionKey(session.session_key);
  return parsed.channel !== "automation"
    && !(parsed.channel === "internal" && parsed.ref === "automation-inbox")
    && session.options.created_by !== "automation_delivery";
}

function buildDeliveryAgentOptions(
  sessions: AgentSession[],
  agentId: string,
  agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  const normalizedAgentId = agentId.trim();
  if (!normalizedAgentId) {
    return [];
  }
  const agentName = agentNameById.get(normalizedAgentId) || normalizedAgentId;
  const options: TaskDialogSessionOption[] = [];
  const seen = new Set<string>();
  sessions.filter((session) => (
    session.agent_id === normalizedAgentId
    && isAgentDMSession(session)
    && isUserVisibleDeliverySession(session)
    && isAvailableDeliverySession(session)
  )).forEach((session) => {
    if (seen.has(session.session_key)) {
      return;
    }
    seen.add(session.session_key);
    const channelLabel = getExternalSessionDisplayLabel(
      session.channel_type,
      session.session_key,
      session.external_identity,
    );
    options.push({
      badge: channelLabel ? `IM · ${channelLabel}` : null,
      label: formatSessionLabel(
        session.title?.trim() || unnamedSessionLabel,
        agentName,
      ),
      sessionKey: session.session_key,
      value: session.session_key,
    });
  });
  return options;
}

function buildDeliveryRoomOptions(
  sessions: AgentSession[],
  roomId: string,
  roomNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  const normalizedRoomId = roomId.trim();
  if (!normalizedRoomId) {
    return [];
  }
  const roomName = roomNameById.get(normalizedRoomId) || normalizedRoomId;
  const seen = new Set<string>();
  const options: TaskDialogSessionOption[] = [];
  sessions.filter((session) => (
    session.room_id === normalizedRoomId
    && isRoomMemberSession(session)
    && Boolean(session.conversation_id)
  )).forEach((session) => {
    const conversationId = session.conversation_id?.trim() || "";
    if (!conversationId || seen.has(conversationId)) {
      return;
    }
    seen.add(conversationId);
    const sharedSessionKey = buildRoomSharedSessionKey(conversationId);
    options.push({
      badge: "Room",
      label: `${roomName} · ${session.title?.trim() || unnamedSessionLabel}`,
      sessionKey: sharedSessionKey,
      value: sharedSessionKey,
    });
  });
  return options;
}

function buildRoomSessionOptions(
  contexts: RoomContextAggregate[],
  _agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  return contexts.filter((context) => (
    context.room.room_type === "room"
    && context.conversation.id.trim()
    && context.sessions.length > 0
  )).map((context) => {
    const sessionKey = buildRoomSharedSessionKey(context.conversation.id);
    const roomName = context.room.name?.trim() || context.room.id;
    const conversationName = context.conversation.title?.trim()
      || unnamedSessionLabel;
    return {
      badge: "Room",
      label: `${roomName} · ${conversationName}`,
      sessionKey,
      value: sessionKey,
    };
  });
}
