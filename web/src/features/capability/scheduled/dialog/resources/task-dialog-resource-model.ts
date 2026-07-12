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
        && (form.executionMode === "existing" || form.replyMode === "selected"),
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
  ) => TaskDialogSessionData
> = {
  agent: ({ agentSessions }, agentNameById) => ({
    options: buildAgentSessionOptions(agentSessions.items, agentNameById),
    status: resourceStatus(agentSessions),
  }),
  room: ({ roomContexts }, agentNameById) => ({
    options: buildRoomSessionOptions(roomContexts.items, agentNameById),
    status: resourceStatus(roomContexts),
  }),
};

export function buildTaskDialogResourceKeys(
  form: TaskFormDraft,
  isOpen: boolean,
): TaskDialogResourceKeys {
  return {
    agents: isOpen ? OPEN_RESOURCE_KEY : null,
    rooms: isOpen && form.targetType === "room" ? OPEN_RESOURCE_KEY : null,
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

export function buildTaskDialogSessionData(
  targetType: TargetType,
  resources: TaskDialogSessionResources,
  agentNameById: Map<string, string>,
): TaskDialogSessionData {
  return SESSION_DATA_BUILDERS[targetType](resources, agentNameById);
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
