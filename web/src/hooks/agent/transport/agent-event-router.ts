/**
 * INPUT: 未知 WebSocket 事件、当前 Session/Room scope 与 durable 游标。
 * OUTPUT: 先丢弃不新于 Room 权威快照的事件，再按唯一处理器路由其余事件。
 * POS: Agent transport 信封校验、重放栅栏与事件所有权边界。
 */
import { parseEventMessage } from "@/lib/websocket/protocol/event-message";
import type { EventMessage } from "@/types/generated/protocol";

import type {
  AgentEventContext,
  AgentEventHandler,
  AgentEventHandlerMap,
} from "./agent-event-context";
import { AGENT_MESSAGE_EVENT_HANDLERS } from "./handlers/agent-message-event-handlers";
import { AGENT_PERMISSION_EVENT_HANDLERS } from "./handlers/permission/permission-event-handlers";
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

function isSupersededRoomEvent(
  event: EventMessage,
  context: AgentEventContext,
): boolean {
  return Boolean(
    context.scope.roomId
    && event.room_id === context.scope.roomId
    && typeof event.room_seq === "number"
    && event.room_seq <= context.transport.roomSeqCursorRef.current,
  );
}

/**
 * WebSocket 层只校验信封并路由，业务事件由各自处理器维护。
 * 未知事件保持忽略，允许后端先发布不影响旧前端的新事件。
 */
export function routeAgentConversationEvent(
  backendMessage: unknown,
  context: AgentEventContext,
): void {
  const event = parseEventMessage(backendMessage);
  if (!event) {
    console.warn(
      "[agent-event-router] Received unexpected message shape:",
      backendMessage,
    );
    return;
  }

  if (isSupersededRoomEvent(event, context)) {
    return;
  }

  updateEventCursors(event, context);
  AGENT_EVENT_HANDLERS.get(event.event_type)?.(
    event,
    context,
  );
}
