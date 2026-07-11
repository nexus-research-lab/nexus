import type {
  AssistantMessage,
  Message,
  StreamMessage,
} from "@/types";

import { normalizeAssistantMessage } from "../message/assistant-message-model";
import { upsertMessage } from "../message/message-collection-model";
import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "./agent-event-context";

const handleStream: AgentEventHandler = (event, context) => {
  const payload = event.data as StreamMessage;
  const messageSessionKey = payload?.session_key || event.session_key || null;
  if (
    !payload ||
    !messageSessionKey ||
    !context.scope.isCurrentSessionEvent(messageSessionKey)
  ) {
    return;
  }

  context.callbacks.enqueueStreamPayload(payload);
};

const handleMessage: AgentEventHandler = (event, context) => {
  const payload = event.data as Message;
  const messageSessionKey = payload?.session_key || event.session_key || null;
  if (!payload || !messageSessionKey) {
    return;
  }

  const message = event.delivery_mode
    ? { ...payload, delivery_mode: event.delivery_mode }
    : payload;
  if (!context.scope.isCurrentSessionEvent(messageSessionKey)) {
    // 后台只缓存可恢复消息，瞬时消息不能跨会话继续展示。
    if (event.delivery_mode !== "ephemeral") {
      context.callbacks.onBackgroundMessage(messageSessionKey, message);
    }
    return;
  }

  const normalizedMessage = message.role === "assistant"
    ? normalizeAssistantMessage(message as AssistantMessage)
    : message;
  context.state.setMessages((currentMessages) => (
    upsertMessage(currentMessages, normalizedMessage)
  ));
  if (normalizedMessage.role === "assistant") {
    context.runtime.trackAssistantMessage(
      normalizedMessage as AssistantMessage,
    );
  }
};

export const AGENT_MESSAGE_EVENT_HANDLERS: AgentEventHandlerMap = {
  message: handleMessage,
  stream: handleStream,
};
