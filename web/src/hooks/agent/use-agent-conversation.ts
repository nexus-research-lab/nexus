import { useCallback, useEffect, useMemo, useRef, useState, startTransition } from 'react';
import { getAgentWsUrl } from '@/config/options';
import { areEquivalentSessionKeys } from '@/lib/session-key';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { EventMessage, Message } from '@/types';
import { PermissionDecisionPayload } from '@/types/permission';
import {
  AgentConversationActionContext,
  AgentConversationLifecycleContext,
  AgentThinkingPayload,
  RoomEventPayload,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from '@/types/agent-conversation';
import { AssistantMessage, AssistantMessageStatus } from '@/types';
import { upsertMessage } from './message-helpers';
import {
  clearAgentSession,
  loadAgentSession,
  resetAgentSession,
  startAgentSession,
} from './conversation-lifecycle';
import { applyStreamMessage } from './message-helpers';
import { handleAgentConversationWebSocketMessage } from './websocket-event-handler';
import {
  sendSessionMessage,
  sendSessionPermissionResponse,
  stopSessionGeneration,
} from './conversation-actions';

interface ActiveMessageTracker {
  round_id: string;
  status: AssistantMessageStatus;
}

export function useAgentConversation(options: UseAgentConversationOptions = {}): UseAgentConversationReturn {
  const ws_url = options.ws_url || getAgentWsUrl();
  const agent_id = options.agent_id;
  const room_id = options.room_id;
  const conversation_id = options.conversation_id;
  const chat_type = options.chat_type;
  const on_error = options.on_error;
  const on_room_event_callback = options.on_room_event;
  const apply_workspace_event = useWorkspaceLiveStore((state) => state.apply_event);

  const [messages, set_messages] = useState<Message[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const [session_key, set_session_key] = useState<string | null>(null);
  const [pending_permissions, set_pending_permissions] = useState<UseAgentConversationReturn['pending_permissions']>([]);
  const [agent_thinking, set_agent_thinking] = useState<AgentThinkingPayload | null>(null);

  const active_session_key_ref = useRef<string | null>(null);
  const load_request_id_ref = useRef(0);
  const room_seq_cursor_ref = useRef(0);
  const pending_round_ids_ref = useRef<Set<string>>(new Set());
  const active_message_tracker_ref = useRef<Map<string, ActiveMessageTracker>>(new Map());
  const pending_permission_count_ref = useRef(0);
  // Per-session message cache: accumulates messages received for non-active sessions
  // so they are not lost when the user switches conversations.
  const bg_message_cache_ref = useRef<Map<string, Message[]>>(new Map());

  // ── Stream batching ──────────────────────────────────────────────────────
  // WebSocket fires on every token (~50-100/sec during streaming).
  // Each token previously called set_messages + set_is_loading = 2 React renders.
  // At 100 tokens/sec that's 200 renders/sec and full CPU saturation.
  //
  // Fix: accumulate stream payloads within an animation frame, flush them all
  // in one setState per frame (≤60 flushes/sec regardless of token rate).
  // startTransition marks the update as non-urgent so React can interrupt it
  // if a higher-priority update (e.g. user keypress) arrives.
  const stream_buffer_ref = useRef<import('@/types').StreamMessage[]>([]);
  const stream_raf_ref = useRef<number | null>(null);

  const lifecycle_context: AgentConversationLifecycleContext = useMemo(() => ({
    active_session_key_ref,
    load_request_id_ref,
    agent_id,
    room_id,
    conversation_id,
    chat_type,
    set_session_key,
    set_messages,
    set_pending_permissions,
    set_is_loading,
    set_error,
    bg_message_cache_ref,
  }), [
    agent_id,
    room_id,
    conversation_id,
    chat_type,
    set_session_key,
    set_messages,
    set_pending_permissions,
    set_is_loading,
    set_error,
  ]);

  const reload_current_session = useCallback(async () => {
    const active_session_key = active_session_key_ref.current;
    if (!active_session_key) {
      return;
    }

    await loadAgentSession(active_session_key, lifecycle_context);
  }, [lifecycle_context]);

  const is_current_session_event = useCallback((incoming_session_key?: string | null) => {
    if (!incoming_session_key) {
      return false;
    }
    return areEquivalentSessionKeys(active_session_key_ref.current, incoming_session_key);
  }, []);

  const on_background_message = useCallback((key: string, message: Message) => {
    const cache = bg_message_cache_ref.current;
    const existing = cache.get(key) ?? [];
    const next = upsertMessage(existing, message);
    cache.set(key, next);
  }, []);

  const on_room_event = useCallback((event_type: string, data: RoomEventPayload) => {
    on_room_event_callback?.(event_type, data);
  }, [on_room_event_callback]);

  const sync_loading_state = useCallback(() => {
    const next_is_loading = (
      pending_round_ids_ref.current.size > 0 ||
      active_message_tracker_ref.current.size > 0 ||
      pending_permission_count_ref.current > 0
    );
    set_is_loading((prev) => (prev === next_is_loading ? prev : next_is_loading));
  }, []);

  const reset_loading_tracker = useCallback(() => {
    pending_round_ids_ref.current.clear();
    active_message_tracker_ref.current.clear();
    pending_permission_count_ref.current = 0;
    set_is_loading(false);
  }, []);

  useEffect(() => {
    pending_permission_count_ref.current = pending_permissions.length;
    sync_loading_state();
  }, [pending_permissions.length, sync_loading_state]);

  const flush_stream_buffer = useCallback(() => {
    stream_raf_ref.current = null;
    const payloads = stream_buffer_ref.current;
    if (payloads.length === 0) return;
    stream_buffer_ref.current = [];

    startTransition(() => {
      set_messages((prev) => {
        let next = prev;
        for (const payload of payloads) {
          next = applyStreamMessage(next, payload);
        }
        return next;
      });
      // 中文注释：流式缓冲可能在 ResultMessage 之后才被刷新。
      // 这里不能盲目把 loading 重新置为 true，否则会把已结束的轮次再次点亮。
      sync_loading_state();
    });
  }, [sync_loading_state]);

  const enqueue_stream_payload = useCallback((payload: import('@/types').StreamMessage) => {
    stream_buffer_ref.current.push(payload);
    if (stream_raf_ref.current === null) {
      stream_raf_ref.current = requestAnimationFrame(flush_stream_buffer);
    }
  }, [flush_stream_buffer]);

  const clear_round_tracking = useCallback((round_id?: string | null) => {
    if (round_id) {
      pending_round_ids_ref.current.delete(round_id);
      for (const [message_id, tracker] of active_message_tracker_ref.current.entries()) {
        if (tracker.round_id === round_id) {
          active_message_tracker_ref.current.delete(message_id);
        }
      }
    }
    sync_loading_state();
  }, [sync_loading_state]);

  const update_message_status = useCallback((
    msg_id: string,
    status: AssistantMessageStatus,
    round_id?: string | null,
  ) => {
    set_messages((prev) =>
      prev.map((m) =>
        m.message_id === msg_id && m.role === 'assistant'
          ? { ...(m as AssistantMessage), stream_status: status }
          : m,
      ),
    );
    const existing_tracker = active_message_tracker_ref.current.get(msg_id);
    const resolved_round_id = round_id ?? existing_tracker?.round_id ?? '';

    if (status === 'done' || status === 'cancelled' || status === 'error') {
      active_message_tracker_ref.current.delete(msg_id);
    } else {
      active_message_tracker_ref.current.set(msg_id, {
        round_id: resolved_round_id,
        status,
      });
    }

    sync_loading_state();
  }, [sync_loading_state]);

  const track_chat_ack = useCallback((ack: import('@/types').ChatAckData, _session_key?: string | null) => {
    pending_round_ids_ref.current.delete(ack.round_id);
    for (const slot of ack.pending ?? []) {
      active_message_tracker_ref.current.set(slot.msg_id, {
        round_id: ack.round_id,
        status: 'pending',
      });
    }
    sync_loading_state();
  }, [sync_loading_state]);

  const track_assistant_message = useCallback((message: AssistantMessage) => {
    pending_round_ids_ref.current.delete(message.round_id);

    if (
      message.stream_status === 'cancelled' ||
      message.stream_status === 'error' ||
      message.stream_status === 'done' ||
      message.is_complete ||
      message.stop_reason
    ) {
      active_message_tracker_ref.current.delete(message.message_id);
    } else {
      active_message_tracker_ref.current.set(message.message_id, {
        round_id: message.round_id,
        status: message.stream_status ?? 'streaming',
      });
    }

    sync_loading_state();
  }, [sync_loading_state]);

  const track_result_message = useCallback((message: import('@/types').ResultMessage) => {
    // 中文注释：Room 多 Agent 场景下，result.parent_id 直指占位 assistant msg_id，
    // 先按 message_id 清理可避免 round_id 不一致时残留“回复中”状态。
    if (message.parent_id) {
      active_message_tracker_ref.current.delete(message.parent_id);
    }
    clear_round_tracking(message.round_id);
  }, [clear_round_tracking]);

  const handle_websocket_message = useCallback((backend_message: unknown) => {
    const event = backend_message as EventMessage;

    if (
      room_id &&
      event.room_id === room_id &&
      typeof event.room_seq === 'number' &&
      event.room_seq > room_seq_cursor_ref.current
    ) {
      room_seq_cursor_ref.current = event.room_seq;
    }

    if (
      event.event_type === 'room_resync_required' &&
      event.room_id === room_id
    ) {
      const latest_room_seq = event.data?.latest_room_seq;
      if (typeof latest_room_seq === 'number') {
        room_seq_cursor_ref.current = Math.max(
          room_seq_cursor_ref.current,
          latest_room_seq,
        );
      }
      on_room_event_callback?.(event.event_type, event.data ?? {});
      void reload_current_session();
      return;
    }

    handleAgentConversationWebSocketMessage({
      backend_message,
      apply_workspace_event,
      is_current_session_event,
      set_error,
      set_is_loading,
      set_messages,
      set_pending_permissions,
      enqueue_stream_payload,
      on_background_message,
      set_agent_thinking,
      on_room_event,
      update_message_status,
      track_chat_ack,
      track_assistant_message,
      track_result_message,
    });
  }, [
    apply_workspace_event,
    is_current_session_event,
    enqueue_stream_payload,
    on_background_message,
    on_room_event,
    on_room_event_callback,
    room_id,
    reload_current_session,
    track_assistant_message,
    track_chat_ack,
    track_result_message,
    update_message_status,
  ]);

  useEffect(() => {
    // 会话切换后，上一条会话的运行态不应污染当前面板。
    reset_loading_tracker();
  }, [reset_loading_tracker, session_key]);

  // Cancel any pending rAF flush on unmount to prevent setState after unmount
  useEffect(() => {
    return () => {
      if (stream_raf_ref.current !== null) {
        cancelAnimationFrame(stream_raf_ref.current);
        stream_raf_ref.current = null;
      }
    };
  }, []);

  const has_connected_ref = useRef(false);

  const { state: ws_state, send: ws_send } = useWebSocket({
    url: ws_url,
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_websocket_message,
    on_error: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!has_connected_ref.current) {
        console.debug('[useAgentConversation] Ignored transient WebSocket error before first successful connection', event);
        return;
      }

      const error_message = 'WebSocket error occurred';
      console.error('[useAgentConversation] WebSocket error:', event);
      set_error(error_message);
      on_error?.(new Error(error_message));
    },
  });

  const previous_ws_state_ref = useRef(ws_state);

  useEffect(() => {
    const previous_ws_state = previous_ws_state_ref.current;
    if (ws_state === 'connected') {
      has_connected_ref.current = true;
      set_error(null);

      // 中文注释：WebSocket 短暂重连后，旧 sender 推送的增量消息可能已经落库但未送达当前页面。
      // 这里在重连成功后主动回拉一次当前会话，补齐断线期间遗漏的消息与工具结果。
      if (previous_ws_state !== 'connected') {
        void reload_current_session();
      }
    }
    previous_ws_state_ref.current = ws_state;
  }, [reload_current_session, ws_state]);

  useEffect(() => {
    if (!agent_id || ws_state !== 'connected') {
      return;
    }

    ws_send({
      type: 'subscribe_workspace',
      agent_id,
    });

    return () => {
      ws_send({
        type: 'unsubscribe_workspace',
        agent_id,
      });
    };
  }, [agent_id, ws_send, ws_state]);

  useEffect(() => {
    if (!session_key || ws_state !== 'connected') {
      return;
    }

    // 中文注释：WebSocket 重连后，后端需要重新知道“当前这个连接服务哪个 session”，
    // 否则挂起中的权限请求无法重投到新连接。
    ws_send({
      type: 'bind_session',
      session_key,
      ...(agent_id ? { agent_id } : {}),
      ...(room_id ? { room_id } : {}),
      ...(conversation_id ? { conversation_id } : {}),
    });
  }, [agent_id, conversation_id, room_id, session_key, ws_send, ws_state]);

  // Subscribe to room-level events (member changes, deletions, etc.) when in a Room context
  useEffect(() => {
    room_seq_cursor_ref.current = 0;
  }, [room_id]);

  useEffect(() => {
    if (!room_id || ws_state !== 'connected') {
      return;
    }

    ws_send({
      type: 'subscribe_room',
      room_id,
      conversation_id,
      ...(room_seq_cursor_ref.current > 0 ? { last_seen_room_seq: room_seq_cursor_ref.current } : {}),
    });

    return () => {
      ws_send({
        type: 'unsubscribe_room',
        room_id,
        conversation_id,
      });
    };
  }, [conversation_id, room_id, ws_send, ws_state]);

  const action_context: AgentConversationActionContext = useMemo(() => ({
    agent_id,
    session_key,
    room_id,
    conversation_id,
    chat_type,
    ws_state,
    ws_send,
    active_session_key_ref,
    pending_permissions,
    messages,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_permissions,
  }), [agent_id, room_id, conversation_id, chat_type, session_key, ws_state, ws_send, pending_permissions, messages, set_error, set_is_loading, set_messages, set_pending_permissions]);

  const send_message = useCallback(async (content: string) => {
    const round_id = await sendSessionMessage(content, action_context);
    if (!round_id) {
      return;
    }

    pending_round_ids_ref.current.add(round_id);
    sync_loading_state();
  }, [action_context, sync_loading_state]);

  const stop_generation = useCallback((msg_id?: string) => {
    stopSessionGeneration(action_context, msg_id);
    if (msg_id) {
      active_message_tracker_ref.current.delete(msg_id);
      sync_loading_state();
      return;
    }

    const latest_user_round_id = [...messages]
      .reverse()
      .find((message) => message.role === 'user')?.round_id;
    clear_round_tracking(latest_user_round_id);
  }, [action_context, clear_round_tracking, messages, sync_loading_state]);

  const send_permission_response = useCallback((payload: PermissionDecisionPayload) => {
    return sendSessionPermissionResponse(payload, action_context);
  }, [action_context]);

  const start_session = useCallback(() => {
    startAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  const load_session = useCallback(async (id: string): Promise<void> => {
    await loadAgentSession(id, lifecycle_context);
  }, [lifecycle_context]);

  const clear_session = useCallback(() => {
    clearAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  const bind_session_key = useCallback((key: string | null) => {
    active_session_key_ref.current = key;
    set_session_key(key);
    if (!key) {
      set_pending_permissions([]);
    }
  }, []);

  const reset_session = useCallback(() => {
    resetAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  return {
    error,
    messages,
    session_key,
    ws_state,
    is_loading,
    pending_permissions,
    agent_thinking,
    send_message,
    bind_session_key,
    start_session,
    load_session,
    clear_session,
    reset_session,
    stop_generation,
    send_permission_response,
  };
}

export type { UseAgentConversationOptions, UseAgentConversationReturn } from '@/types/agent-conversation';
