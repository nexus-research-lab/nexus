import type { RoomEventPayload } from "@/types/agent/agent-conversation";
import type { WorkspaceEventPayload } from "@/types/app/workspace-live";

import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";

const handleAgentRuntimeEvent: AgentEventHandler = (event, context) => {
  const payload = event.data as
    | { agent_id?: string; running_task_count?: number; status?: string }
    | undefined;
  if (
    payload?.agent_id === context.scope.agentId
    && payload.running_task_count === 0
    && payload.status !== "running"
  ) {
    context.callbacks.settleAgentWorkspaceWrites(payload.agent_id);
  }
};

const handleWorkspaceEvent: AgentEventHandler = (event, context) => {
  const payload = event.data as WorkspaceEventPayload;
  if (payload?.agent_id && payload.path) {
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
