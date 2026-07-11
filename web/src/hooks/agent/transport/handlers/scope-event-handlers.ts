import { parseAgentRuntimeStatus } from "@/lib/agent-runtime-status";
import {
  asUnknownRecord,
  readNumber,
  readString,
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

function parseWorkspaceEventPayload(value: unknown): WorkspaceEventPayload | null {
  const record = asUnknownRecord(value);
  if (!record) {
    return null;
  }
  const type = readString(record, "type") as WorkspaceEventPayload["type"] | null;
  const source = readString(record, "source") as WorkspaceEventPayload["source"] | null;
  if (
    !type
    || !WORKSPACE_EVENT_TYPES.has(type)
    || !source
    || !WORKSPACE_EVENT_SOURCES.has(source)
    || !readString(record, "agent_id")
    || !readString(record, "path")
    || readNumber(record, "version") === null
    || !readString(record, "timestamp")
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
