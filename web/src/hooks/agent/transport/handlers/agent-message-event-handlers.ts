/**
 * INPUT: 当前会话上有序到达的 WebSocket stream 与完整 message 事件。
 * OUTPUT: canonical snapshot 前同步 flush 更早 patch 的会话内消息更新。
 * POS: Agent realtime 事件进入消息集合前的顺序边界。
 */
import {
  parseConversationMessage,
  parseStreamMessage,
} from "@/lib/conversation/message-protocol";
import type { AssistantMessage } from "@/types/conversation/message/entity";

import {
  normalizeAssistantMessage,
  resolveAssistantFailureCode,
  resolveAssistantResultErrorBannerMessage,
} from "../../message/assistant-message-model";
import { upsertRealtimeMessage } from "../../message/message-collection-model";
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
  context.state.reliability.observeRecovery({
    agent_round_id: payload.agent_round_id,
    kind: "round_progress",
    round_id: payload.round_id,
    session_key: messageSessionKey,
  });
  context.runtime.trackStreamExecution(payload);
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
    // 后台只缓存可恢复消息，round 临时态和当前界面通知都不能跨会话展示。
    if (event.delivery_mode === "durable") {
      context.callbacks.onBackgroundMessage(messageSessionKey, message);
    }
    return;
  }

  const normalizedMessage = message.role === "assistant"
    ? normalizeAssistantMessage(message as AssistantMessage)
    : message;
  const isProviderRetry = normalizedMessage.role === "assistant"
    && normalizedMessage.content.some((block) => (
      block.type === "system_event" && block.subtype === "api_retry"
    ));
  if (
    context.scope.chatType === "dm"
    && isProviderRetry
    && normalizedMessage.round_id?.trim()
  ) {
    context.state.reliability.observeProviderRetry({
      agent_round_id: normalizedMessage.agent_round_id,
      round_id: normalizedMessage.round_id,
      session_key: messageSessionKey,
    });
  } else if (normalizedMessage.round_id?.trim()) {
    context.state.reliability.observeRecovery({
      agent_round_id: normalizedMessage.agent_round_id,
      kind: "round_progress",
      round_id: normalizedMessage.round_id,
      session_key: messageSessionKey,
    });
  }
  // 同一 WebSocket 上 message 快照晚于此前 stream event。先同步清空 RAF
  // 缓冲，才能保证旧累计 patch 不会在下一帧把较新 streaming 快照缩短。
  context.callbacks.flushStreamPayloads();
  context.state.setMessages((currentMessages) => (
    upsertRealtimeMessage(currentMessages, normalizedMessage)
  ));
  context.callbacks.settleLiveMessageSnapshot(normalizedMessage);
  context.callbacks?.onRoomEvent?.(event.event_type, event.data ?? {});
  if (normalizedMessage.role === "assistant") {
    context.runtime.trackAssistantMessage(
      normalizedMessage as AssistantMessage,
    );
    const resultError = resolveAssistantResultErrorBannerMessage(
      normalizedMessage as AssistantMessage,
    );
    if (resultError && context.scope.chatType === "dm") {
      console.error("[useAgentConversation] Agent round failed:", resultError);
      context.state.reliability.reportFailure({
        agent_round_id: normalizedMessage.agent_round_id,
        code: resolveAssistantFailureCode(normalizedMessage as AssistantMessage),
        round_id: normalizedMessage.round_id,
        session_key: messageSessionKey,
      });
    }
  }
};

export const AGENT_MESSAGE_EVENT_HANDLERS: AgentEventHandlerMap = {
  message: handleMessage,
  stream: handleStream,
};
