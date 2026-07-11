import type { EventMessage } from "@/types";

import type {
  AgentEventContext,
  AgentEventHandler,
  AgentEventHandlerMap,
} from "./agent-event-context";
import { AGENT_MESSAGE_EVENT_HANDLERS } from "./handlers/agent-message-event-handlers";
import { AGENT_PERMISSION_EVENT_HANDLERS } from "./handlers/permission-event-handlers";
import { AGENT_RESYNC_EVENT_HANDLERS } from "./handlers/resync-event-handlers";
import { AGENT_SCOPE_EVENT_HANDLERS } from "./handlers/scope-event-handlers";
import { AGENT_SESSION_EVENT_HANDLERS } from "./handlers/session-event-handlers";

function registerEventHandlers(
  handlerMaps: AgentEventHandlerMap[],
): Map<string, AgentEventHandler> {
  const handlers = new Map<string, AgentEventHandler>();
  for (const handlerMap of handlerMaps) {
    for (const [eventType, handler] of Object.entries(handlerMap)) {
      if (handlers.has(eventType)) {
        throw new Error(`Agent event handler 重复注册: ${eventType}`);
      }
      handlers.set(eventType, handler);
    }
  }
  return handlers;
}

const AGENT_EVENT_HANDLERS = registerEventHandlers([
  AGENT_MESSAGE_EVENT_HANDLERS,
  AGENT_PERMISSION_EVENT_HANDLERS,
  AGENT_RESYNC_EVENT_HANDLERS,
  AGENT_SCOPE_EVENT_HANDLERS,
  AGENT_SESSION_EVENT_HANDLERS,
]);

function isEventMessage(data: unknown): data is EventMessage {
  if (typeof data !== "object" || data === null) {
    return false;
  }
  const message = data as Record<string, unknown>;
  return (
    typeof message.event_type === "string" &&
    typeof message.protocol_version === "number"
  );
}

function updateEventCursors(
  event: EventMessage,
  context: AgentEventContext,
): void {
  if (
    context.scope.sessionKey &&
    event.session_key === context.scope.sessionKey &&
    typeof event.session_seq === "number"
  ) {
    context.transport.sessionSeqCursorRef.current = Math.max(
      context.transport.sessionSeqCursorRef.current,
      event.session_seq,
    );
  }
  if (
    context.scope.roomId &&
    event.room_id === context.scope.roomId &&
    typeof event.room_seq === "number"
  ) {
    context.transport.roomSeqCursorRef.current = Math.max(
      context.transport.roomSeqCursorRef.current,
      event.room_seq,
    );
  }
}

/**
 * WebSocket 层只校验信封并路由，业务事件由各自处理器维护。
 * 未知事件保持忽略，允许后端先发布不影响旧前端的新事件。
 */
export function routeAgentConversationEvent(
  backendMessage: unknown,
  context: AgentEventContext,
): void {
  if (!isEventMessage(backendMessage)) {
    console.warn(
      "[agent-event-router] Received unexpected message shape:",
      backendMessage,
    );
    return;
  }

  updateEventCursors(backendMessage, context);
  AGENT_EVENT_HANDLERS.get(backendMessage.event_type)?.(
    backendMessage,
    context,
  );
}
