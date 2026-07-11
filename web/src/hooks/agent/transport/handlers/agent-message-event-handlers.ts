import {
  parseConversationMessage,
  parseStreamMessage,
} from "@/lib/conversation/message-protocol";
import type { AssistantMessage } from "@/types/conversation/message/entity";

import { normalizeAssistantMessage } from "../../message/assistant-message-model";
import { upsertMessage } from "../../message/message-collection-model";
import type {
  AgentEventHandler,
  AgentEventHandlerMap,
} from "../agent-event-context";

const handleStream: AgentEventHandler = (event, context) => {
  const payload = parseStreamMessage(event.data, event.session_key);
  const messageSessionKey = payload?.session_key ?? null;
  if (
    !payload
    || !messageSessionKey
    || !context.scope.isCurrentSessionEvent(messageSessionKey)
  ) {
    return;
  }
  context.callbacks.enqueueStreamPayload(payload);
};

const handleMessage: AgentEventHandler = (event, context) => {
  const message = parseConversationMessage(event.data, {
    deliveryMode: event.delivery_mode,
    sessionKey: event.session_key,
  });
  const messageSessionKey = message?.session_key ?? null;
  if (!message || !messageSessionKey) {
    return;
  }
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
