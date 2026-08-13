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
import {
  buildRoomSessionSelections,
  formatSessionLabel,
} from "../schedule/task-schedule-time";
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
  return rooms.map((room) => ({
    label: room.room.name?.trim() || room.room.id,
    value: room.room.id,
  }));
}

export function buildExecutionRoomOptions(
  rooms: RoomAggregate[],
): TaskDialogLabelOption[] {
  return rooms.filter((room) => room.members.some((member) => (
    member.member_type === "agent" && !member.participation_paused
  ))).map((room) => ({
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
    session.conversation_id === conversationId
    && Boolean(session.room_id)
  ))?.room_id?.trim() || "";
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

function buildAgentSessionOptions(
  sessions: AgentSession[],
  agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  return sessions.filter((session) => {
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
      agentId: session.agent_id,
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
      agentId: normalizedAgentId,
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
    && Boolean(session.conversation_id)
  )).forEach((session) => {
    const conversationId = session.conversation_id?.trim() || "";
    if (!conversationId || seen.has(conversationId)) {
      return;
    }
    seen.add(conversationId);
    const sharedSessionKey = buildRoomSharedSessionKey(conversationId);
    options.push({
      agentId: session.agent_id,
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
  agentNameById: Map<string, string>,
  unnamedSessionLabel: string,
): TaskDialogSessionOption[] {
  return buildRoomSessionSelections(
    contexts,
    agentNameById,
    unnamedSessionLabel,
  ).map((option) => ({
    agentId: option.agent_id,
    label: option.label,
    sessionKey: option.session_key,
    value: option.value,
  }));
}
