import { parseAgentRuntimeStatus } from "@/lib/agent-runtime-status";
import {
  asUnknownRecord,
  hasFiniteNumberFields,
  hasNonEmptyStringFields,
  readStringFromSet,
} from "@/lib/unknown-value";
import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { WorkspaceEventPayload } from "@/types/app/workspace-live";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";

const WORKSPACE_EVENT_TYPES = new Set<WorkspaceEventPayload["type"]>([
  "file_deleted",
  "file_write_delta",
  "file_write_end",
  "file_write_start",
]);
const WORKSPACE_EVENT_SOURCES = new Set<WorkspaceEventPayload["source"]>([
  "agent",
  "api",
  "system",
  "unknown",
]);
const WORKSPACE_EVENT_REQUIRED_STRING_FIELDS = [
  "agent_id",
  "path",
  "timestamp",
] as const;
const WORKSPACE_EVENT_REQUIRED_NUMBER_FIELDS = ["version"] as const;

function parseWorkspaceEventPayload(value: unknown): WorkspaceEventPayload | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const type = readStringFromSet(record, "type", WORKSPACE_EVENT_TYPES);
  const source = readStringFromSet(record, "source", WORKSPACE_EVENT_SOURCES);
  if (
    !type
    || !source
    || !hasNonEmptyStringFields(record, WORKSPACE_EVENT_REQUIRED_STRING_FIELDS)
    || !hasFiniteNumberFields(record, WORKSPACE_EVENT_REQUIRED_NUMBER_FIELDS)
  ) {
    return null;
  }
  return record as unknown as WorkspaceEventPayload;
}

const handleAgentRuntimeEvent: AgentEventHandler = (event, context) => {
  const payload = parseAgentRuntimeStatus(event.data);
  if (
    payload?.agent_id === context.scope.agentId
    && payload.running_task_count === 0
    && payload.status !== "running"
  ) {
    context.callbacks.settleAgentWorkspaceWrites(payload.agent_id);
  }
};

const handleWorkspaceEvent: AgentEventHandler = (event, context) => {
  const payload = parseWorkspaceEventPayload(event.data);
  if (payload) {
    context.callbacks.applyWorkspaceEvent(payload);
  }
};

const handleRoomEvent: AgentEventHandler = (event, context) => {
  if (!context.scope.isCurrentRoomEvent(event.room_id)) {
    return;
  }
  context.callbacks.onRoomEvent(
    event.event_type,
    (event.data ?? {}) as RoomEventPayload,
  );
};

export const AGENT_SCOPE_EVENT_HANDLERS: AgentEventHandlerMap = {
  agent_runtime_event: handleAgentRuntimeEvent,
  room_deleted: handleRoomEvent,
  room_directed_message: handleRoomEvent,
  room_directed_message_consumed: handleRoomEvent,
  room_member_added: handleRoomEvent,
  room_member_removed: handleRoomEvent,
  workspace_event: handleWorkspaceEvent,
};
