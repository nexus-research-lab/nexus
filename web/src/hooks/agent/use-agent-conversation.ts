import { useCallback, useEffect, useMemo, useRef, useState, startTransition } from 'react';
import { getAgentWsUrl } from '@/config/options';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { Message } from '@/types';
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
  clearAgentConversation,
  loadAgentConversation,
  resetAgentConversation,
  startAgentConversation,
} from './conversation-lifecycle';
import { applyStreamMessage } from './message-helpers';
import { handleAgentConversationWebSocketMessage } from './websocket-event-handler';
import {
  deleteConversationRound,
  regenerateConversationRound,
  sendConversationMessage,
  sendConversationPermissionResponse,
  stopConversationGeneration,
} from './conversation-actions';

export function useAgentConversation(options: UseAgentConversationOptions = {}): UseAgentConversationReturn {
  const ws_url = options.ws_url || getAgentWsUrl();
  const apply_workspace_event = useWorkspaceLiveStore((state) => state.apply_event);

  const [messages, set_messages] = useState<Message[]>([]);
  const [is_loading, set_is_loading] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const [session_key, set_session_key] = useState<string | null>(null);
  const [pending_permission, set_pending_permission] = useState<UseAgentConversationReturn['pending_permission']>(null);
  const [agent_thinking, set_agent_thinking] = useState<AgentThinkingPayload | null>(null);

  const active_conversation_key_ref = useRef<string | null>(null);
  const load_request_id_ref = useRef(0);
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
      set_is_loading(true);
    });
  }, []);

  const enqueue_stream_payload = useCallback((payload: import('@/types').StreamMessage) => {
    stream_buffer_ref.current.push(payload);
    if (stream_raf_ref.current === null) {
      stream_raf_ref.current = requestAnimationFrame(flush_stream_buffer);
    }
  }, [flush_stream_buffer]);
  const lifecycle_context: AgentConversationLifecycleContext = useMemo(() => ({
    active_conversation_key_ref,
    load_request_id_ref,
    set_conversation_key: set_session_key,
    set_messages,
    set_pending_permission,
    set_is_loading,
    set_error,
    bg_message_cache_ref,
  }), [set_session_key, set_messages, set_pending_permission, set_is_loading, set_error]);

  const is_current_session_event = useCallback((incoming_session_key?: string | null) => {
    if (!incoming_session_key) {
      return false;
    }
    return active_conversation_key_ref.current === incoming_session_key;
  }, []);

  const on_background_message = useCallback((key: string, message: Message) => {
    const cache = bg_message_cache_ref.current;
    const existing = cache.get(key) ?? [];
    const next = upsertMessage(existing, message);
    cache.set(key, next);
  }, []);

  const on_room_event = useCallback((event_type: string, data: RoomEventPayload) => {
    options.on_room_event?.(event_type, data);
  }, [options]);

  const update_message_status = useCallback((msg_id: string, status: AssistantMessageStatus) => {
    set_messages((prev) =>
      prev.map((m) =>
        m.message_id === msg_id && m.role === 'assistant'
          ? { ...(m as AssistantMessage), stream_status: status }
          : m,
      ),
    );
  }, []);

  const handle_websocket_message = useCallback((backend_message: unknown) => {
    handleAgentConversationWebSocketMessage({
      backend_message,
      apply_workspace_event,
      is_current_session_event,
      set_error,
      set_is_loading,
      set_messages,
      set_pending_permission,
      enqueue_stream_payload,
      on_background_message,
      set_agent_thinking,
      on_room_event,
      update_message_status,
    });
  }, [apply_workspace_event, is_current_session_event, enqueue_stream_payload, on_background_message, on_room_event, update_message_status]);

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
      options.on_error?.(new Error(error_message));
    },
  });

  useEffect(() => {
    if (ws_state === 'connected') {
      has_connected_ref.current = true;
      set_error(null);
    }
  }, [ws_state]);

  useEffect(() => {
    const agent_id = options.agent_id;
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
  }, [options.agent_id, ws_send, ws_state]);

  // Subscribe to room-level events (member changes, deletions, etc.) when in a Room context
  useEffect(() => {
    const room_id = options.room_id;
    if (!room_id || ws_state !== 'connected') {
      return;
    }

    ws_send({ type: 'subscribe_room', room_id });

    return () => {
      ws_send({ type: 'unsubscribe_room', room_id });
    };
  }, [options.room_id, ws_send, ws_state]);

  const action_context: AgentConversationActionContext = useMemo(() => ({
    agent_id: options.agent_id,
    session_key,
    room_id: options.room_id,
    conversation_id: options.conversation_id,
    chat_type: options.chat_type,
    ws_state,
    ws_send,
    active_conversation_key_ref,
    pending_permission,
    messages,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_permission,
  }), [options.agent_id, options.room_id, options.conversation_id, options.chat_type, session_key, ws_state, ws_send, pending_permission, messages, set_error, set_is_loading, set_messages, set_pending_permission]);

  const send_message = useCallback(async (content: string) => {
    await sendConversationMessage(content, action_context);
  }, [action_context]);

  const stop_generation = useCallback((msg_id?: string) => {
    stopConversationGeneration(action_context, msg_id);
  }, [action_context]);

  const send_permission_response = useCallback((payload: PermissionDecisionPayload) => {
    return sendConversationPermissionResponse(payload, action_context);
  }, [action_context]);

  const regenerate = useCallback(async (round_id: string) => {
    await regenerateConversationRound(round_id, action_context);
  }, [action_context]);

  const delete_round = useCallback(async (round_id: string) => {
    await deleteConversationRound(round_id, action_context);
  }, [action_context]);

  const start_conversation = useCallback(() => {
    startAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  const load_conversation = useCallback(async (id: string): Promise<void> => {
    await loadAgentConversation(id, lifecycle_context);
  }, [lifecycle_context]);

  const clear_conversation = useCallback(() => {
    clearAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  const bind_conversation_key = useCallback((key: string | null) => {
    active_conversation_key_ref.current = key;
    set_session_key(key);
    if (!key) {
      set_pending_permission(null);
    }
  }, []);

  const reset_conversation = useCallback(() => {
    resetAgentConversation(lifecycle_context);
  }, [lifecycle_context]);

  return {
    error,
    messages,
    session_key,
    ws_state,
    is_loading,
    pending_permission,
    agent_thinking,
    send_message,
    bind_conversation_key,
    start_conversation,
    load_conversation,
    clear_conversation,
    reset_conversation,
    stop_generation,
    delete_round,
    regenerate,
    send_permission_response,
  };
}

export type { UseAgentConversationOptions, UseAgentConversationReturn } from '@/types/agent-conversation';
