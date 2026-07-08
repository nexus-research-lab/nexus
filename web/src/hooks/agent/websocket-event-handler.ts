import {
  AgentRoundStatusEventPayload,
  AssistantMessage,
  ChatAckData,
  EventMessage,
  Message,
  RoundStatusEventPayload,
  SessionStatusEventPayload,
  StreamMessage,
} from "@/types";
import {
  HandleAgentConversationWebSocketMessageParams,
  InputQueueEventPayload,
  RoomEventPayload,
} from "@/types/agent/agent-conversation";
import { WorkspaceEventPayload } from "@/types/app/workspace-live";
import {
  applyStreamMessage,
  normalizeAssistantMessage,
  upsertMessage,
} from "./message-helpers";
import {
  buildRoomSubscriptionMessage,
  buildSessionBindMessage,
} from "./conversation-actions";

function isEventMessage(data: unknown): data is EventMessage {
  if (typeof data !== "object" || data === null) {
    return false;
  }
  const msg = data as Record<string, unknown>;
  return (
    typeof msg.event_type === "string" &&
    typeof msg.protocol_version === "number"
  );
}

function eventRoundId(event: EventMessage): string | null {
  const dataRoundId = typeof event.data?.round_id === "string"
    ? event.data.round_id.trim()
    : "";
  if (dataRoundId) {
    return dataRoundId;
  }
  const envelopeRoundId = typeof event.round_id === "string"
    ? event.round_id.trim()
    : "";
  return envelopeRoundId || null;
}

/**
 * 处理 Agent 会话的 WebSocket 事件。
 */
export function handleAgentConversationWebSocketMessage({
  backend_message: backendMessage,
  agent_id: agentId,
  room_id: roomId,
  conversation_id: conversationId,
  session_key: sessionKey,
  session_seq_cursor_ref: sessionSeqCursorRef,
  room_seq_cursor_ref: roomSeqCursorRef,
  ws_state_ref: wsStateRef,
  ws_send_ref: wsSendRef,
  apply_workspace_event: applyWorkspaceEvent,
  is_current_room_event: isCurrentRoomEvent,
  is_current_session_event: isCurrentSessionEvent,
  set_error: setError,
  set_messages: setMessages,
  set_input_queue_items: setInputQueueItems,
  set_pending_permissions: setPendingPermissions,
  enqueue_stream_payload: enqueueStreamPayload,
  on_background_message: onBackgroundMessage,
  on_room_event: onRoomEvent,
  update_message_status: updateMessageStatus,
  sync_session_status: syncSessionStatus,
  apply_round_status: applyRoundStatus,
  apply_agent_round_status: applyAgentRoundStatus,
  track_chat_ack: trackChatAck,
  reject_chat_ack: rejectChatAck,
  track_assistant_message: trackAssistantMessage,
  remove_rewritten_round: removeRewrittenRound,
  reload_current_session: reloadCurrentSession,
  settleAgentWorkspaceWrites: settleAgentWorkspaceWrites,
}: HandleAgentConversationWebSocketMessageParams): void {
  if (!isEventMessage(backendMessage)) {
    console.warn("[websocket-event-handler] Received unexpected message shape:", backendMessage);
    return;
  }
  const event = backendMessage;
  const incomingSessionKey = event.session_key || null;

  if (
    sessionKey &&
    event.session_key === sessionKey &&
    typeof event.session_seq === "number" &&
    sessionSeqCursorRef &&
    event.session_seq > sessionSeqCursorRef.current
  ) {
    sessionSeqCursorRef.current = event.session_seq;
  }

  if (
    roomId &&
    event.room_id === roomId &&
    typeof event.room_seq === "number" &&
    roomSeqCursorRef &&
    event.room_seq > roomSeqCursorRef.current
  ) {
    roomSeqCursorRef.current = event.room_seq;
  }

  if (
    event.event_type === "room_resync_required" &&
    event.room_id === roomId
  ) {
    const latestRoomSeq = event.data?.latest_room_seq;
    if (typeof latestRoomSeq === "number" && roomSeqCursorRef) {
      roomSeqCursorRef.current = Math.max(
        roomSeqCursorRef.current,
        latestRoomSeq,
      );
    }
    onRoomEvent?.(event.event_type, event.data ?? {});
    void reloadCurrentSession?.().finally(() => {
      if (
        !roomId ||
        wsStateRef?.current !== "connected" ||
        !wsSendRef ||
        !roomSeqCursorRef
      ) {
        return;
      }
      wsSendRef.current(buildRoomSubscriptionMessage({
        type: "subscribe_room",
        room_id: roomId,
        conversation_id: conversationId,
        last_seen_room_seq: roomSeqCursorRef.current,
      }));
    });
    return;
  }

  if (
    event.event_type === "session_resync_required" &&
    event.session_key &&
    isCurrentSessionEvent(event.session_key)
  ) {
    const latestSessionSeq = event.data?.latest_session_seq;
    if (typeof latestSessionSeq === "number" && sessionSeqCursorRef) {
      sessionSeqCursorRef.current = Math.max(
        sessionSeqCursorRef.current,
        latestSessionSeq,
      );
    }
    const reason = typeof event.data?.reason === "string"
      ? event.data.reason
      : "";
    const targetRoundId = typeof event.data?.target_round_id === "string"
      ? event.data.target_round_id.trim()
      : "";
    if (reason === "history_rewrite" && targetRoundId) {
      removeRewrittenRound?.(targetRoundId);
    }
    onRoomEvent?.(event.event_type, event.data ?? {});
    void reloadCurrentSession?.().finally(() => {
      if (
        !sessionKey ||
        wsStateRef?.current !== "connected" ||
        !wsSendRef ||
        !sessionSeqCursorRef
      ) {
        return;
      }
      wsSendRef.current(buildSessionBindMessage({
        session_key: sessionKey,
        last_seen_session_seq: sessionSeqCursorRef.current,
        agent_id: agentId,
        room_id: roomId,
        conversation_id: conversationId,
      }));
    });
    return;
  }

  if (event.event_type === "agent_runtime_event") {
    const payload = event.data as
      | { agent_id?: string; running_task_count?: number; status?: string }
      | undefined;
    if (
      payload?.agent_id &&
      payload.agent_id === agentId &&
      payload.running_task_count === 0 &&
      payload.status !== "running"
    ) {
      settleAgentWorkspaceWrites?.(payload.agent_id);
    }
    return;
  }

  if (event.event_type === "error") {
    if (
      incomingSessionKey &&
      !isCurrentSessionEvent(incomingSessionKey)
    ) {
      return;
    }
    const roundId = eventRoundId(event);
    if (roundId) {
      applyRoundStatus?.(roundId, "error");
    }
    if (event.message_id) {
      updateMessageStatus?.(event.message_id, "error", roundId);
    }
    const message = event.data?.message || "Unknown error";
    const clientRequestId =
      typeof event.data?.client_request_id === "string"
        ? event.data.client_request_id
        : "";
    if (clientRequestId) {
      rejectChatAck?.(clientRequestId, message);
    }
    setError(message);
    return;
  }

  if (event.event_type === "permission_request") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const data = event.data || {};
    setPendingPermissions((prev) => {
      const nextPermission = {
        request_id: data.request_id,
        tool_name: data.tool_name,
        tool_input: data.tool_input || {},
        session_key: incomingSessionKey,
        agent_id: data.agent_id ?? event.agent_id ?? null,
        message_id: data.message_id ?? event.message_id ?? null,
        round_id: data.round_id ?? event.round_id ?? null,
        agent_round_id: data.agent_round_id ?? event.agent_round_id ?? null,
        tool_use_id: data.tool_use_id ?? null,
        interaction_mode:
          data.interaction_mode ??
          (data.tool_name === "AskUserQuestion" ? "question" : "permission"),
        risk_level: data.risk_level,
        risk_label: data.risk_label,
        summary: data.summary,
        suggestions: data.suggestions || [],
        expires_at: data.expires_at,
      };
      return [
        ...prev.filter((item) => item.request_id !== data.request_id),
        nextPermission,
      ];
    });
    return;
  }

  if (event.event_type === "permission_request_resolved") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const requestId =
      typeof event.data?.request_id === "string" ? event.data.request_id : "";
    if (!requestId) {
      return;
    }
    setPendingPermissions((prev) => {
      const next = prev.filter((item) => item.request_id !== requestId);
      return next.length === prev.length ? prev : next;
    });
    return;
  }

  if (event.event_type === "workspace_event") {
    const payload = event.data as WorkspaceEventPayload;
    if (payload?.agent_id && payload?.path) {
      applyWorkspaceEvent(payload);
    }
    return;
  }

  // Room-level events (member changes, room deleted, etc.)
  if (
    event.event_type === "room_member_added" ||
    event.event_type === "room_member_removed" ||
    event.event_type === "room_deleted" ||
    event.event_type === "room_directed_message" ||
    event.event_type === "room_directed_message_consumed" ||
    event.event_type === "room_resync_required" ||
    event.event_type === "session_resync_required"
  ) {
    if (isCurrentRoomEvent && !isCurrentRoomEvent(event.room_id)) {
      return;
    }
    onRoomEvent?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  // sessionStatus: 重连后后端告知该 session 是否仍在生成，恢复/收口 loading 态
  if (event.event_type === "session_status") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const payload = (event.data ?? {}) as SessionStatusEventPayload;
    syncSessionStatus?.(payload);
    return;
  }

  if (event.event_type === "input_queue") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const payload = (event.data ?? {}) as InputQueueEventPayload;
    const items = Array.isArray(payload.items) ? payload.items : [];
    setInputQueueItems?.(items);
    return;
  }

  if (isGoalEvent(event.event_type)) {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    onRoomEvent?.(event.event_type, (event.data ?? {}) as RoomEventPayload);
    return;
  }

  if (event.event_type === "round_status") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const payload = (event.data ?? {}) as RoundStatusEventPayload;
    if (!payload.round_id || !payload.status) {
      return;
    }
    applyRoundStatus?.(payload.round_id, payload.status);
    return;
  }

  // agent_round_status: Room slot 生命周期，只收口对应 slot
  if (event.event_type === "agent_round_status") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const payload = (event.data ?? {}) as AgentRoundStatusEventPayload;
    if (!payload.agent_round_id || !payload.status) {
      return;
    }
    applyAgentRoundStatus?.(payload);
    return;
  }

  // chatAck: 关联 client_request_id、替换 optimistic user message、登记占位槽位
  if (event.event_type === "chat_ack") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const ack = event.data as ChatAckData;
    if (!ack?.round_id) {
      return;
    }
    trackChatAck?.(ack, incomingSessionKey);
    return;
  }

  // streamStart: flip placeholder from pending → streaming
  if (event.event_type === "stream_start") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const msgId = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msgId) {
      updateMessageStatus?.(msgId, "streaming", event.data?.round_id);
    }
    return;
  }

  // streamEnd: mark bubble done
  if (event.event_type === "stream_end") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const msgId = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msgId) {
      updateMessageStatus?.(msgId, "done", event.data?.round_id);
    }
    return;
  }

  // streamCancelled: mark bubble cancelled, stop loading
  if (event.event_type === "stream_cancelled") {
    if (!isCurrentSessionEvent(incomingSessionKey)) {
      return;
    }
    const msgId = (event.message_id || event.data?.msg_id) as
      | string
      | undefined;
    if (msgId) {
      updateMessageStatus?.(msgId, "cancelled", event.data?.round_id);
    }
    return;
  }

  if (event.event_type !== "message") {
    if (event.event_type !== "stream") {
      return;
    }

    const payload = event.data as StreamMessage;
    const messageSessionKey = payload?.session_key || incomingSessionKey;
    if (
      !payload ||
      !messageSessionKey ||
      !isCurrentSessionEvent(messageSessionKey)
    ) {
      return;
    }

    // Route to rAF batch buffer when available (≤60 flushes/sec),
    // otherwise fall back to direct update (e.g. during tests).
    if (enqueueStreamPayload) {
      enqueueStreamPayload(payload);
    } else {
      setMessages((prev) => applyStreamMessage(prev, payload));
    }
    return;
  }

  const payload = event.data as Message;
  const messageSessionKey = payload?.session_key || incomingSessionKey;
  if (!payload || !messageSessionKey) {
    return;
  }

  const payloadWithDeliveryMode: Message = event.delivery_mode
    ? {
        ...payload,
        delivery_mode: event.delivery_mode,
      }
    : payload;

  if (!isCurrentSessionEvent(messageSessionKey)) {
    // 只缓存 durable 消息，ephemeral 仅服务当前活跃轮次展示。
    if (event.delivery_mode !== "ephemeral" && onBackgroundMessage) {
      onBackgroundMessage(messageSessionKey, payloadWithDeliveryMode);
    }
    return;
  }

  const normalizedPayload =
    payloadWithDeliveryMode.role === "assistant"
      ? normalizeAssistantMessage(
          payloadWithDeliveryMode as AssistantMessage,
        )
      : payloadWithDeliveryMode;

  setMessages((prev) => upsertMessage(prev, normalizedPayload));
  if (normalizedPayload.role === "assistant") {
    trackAssistantMessage?.(normalizedPayload as AssistantMessage);
  }
}

function isGoalEvent(eventType: string): boolean {
  return (
    eventType === "goal_created" ||
    eventType === "goal_updated" ||
    eventType === "goal_status_changed" ||
    eventType === "goal_progress" ||
    eventType === "goal_continuation" ||
    eventType === "goal_cleared"
  );
}
