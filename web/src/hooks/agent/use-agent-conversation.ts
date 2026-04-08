import { SetStateAction, useCallback, useEffect, useMemo, useRef, useState, startTransition } from 'react';
import { getAgentWsUrl } from '@/config/options';
import { areEquivalentSessionKeys } from '@/lib/session-key';
import { useWebSocket } from '@/lib/websocket';
import { useWorkspaceLiveStore } from '@/store/workspace-live';
import { EventMessage, Message } from '@/types';
import {
  PendingPermission,
  PermissionDecisionPayload,
  buildPermissionSignature,
} from '@/types/permission';
import {
  AgentConversationActionContext,
  AgentConversationLifecycleContext,
  AgentThinkingPayload,
  RoomEventPayload,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from '@/types/agent-conversation';
import { AssistantMessage, AssistantMessageStatus, RoomPendingAgentSlotState } from '@/types';
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

function isTerminalAssistantStatus(status?: AssistantMessageStatus): boolean {
  return status === 'done' || status === 'cancelled' || status === 'error';
}

function collectCompletedRoundIds(messages: Message[]): Set<string> {
  const completed_round_ids = new Set<string>();
  for (const message of messages) {
    if (message.role === 'result') {
      completed_round_ids.add(message.round_id);
    }
  }
  return completed_round_ids;
}

function collectOpenRoundIds(messages: Message[]): Set<string> {
  const completed_round_ids = collectCompletedRoundIds(messages);
  const open_round_ids = new Set<string>();
  for (const message of messages) {
    if (message.role === 'assistant' && !completed_round_ids.has(message.round_id)) {
      open_round_ids.add(message.round_id);
    }
  }
  return open_round_ids;
}

function filterPendingSlotsFromSnapshot(
  current_slots: RoomPendingAgentSlotState[],
  messages: Message[],
): RoomPendingAgentSlotState[] {
  if (current_slots.length === 0) {
    return current_slots;
  }

  const completed_round_ids = collectCompletedRoundIds(messages);
  const loaded_message_ids = new Set(
    messages
      .filter((message): message is AssistantMessage => message.role === 'assistant')
      .map((message) => message.message_id),
  );

  return current_slots.filter((slot) => (
    !completed_round_ids.has(slot.round_id) &&
    !loaded_message_ids.has(slot.msg_id)
  ));
}

function filterPendingPermissionsFromSnapshot(
  current_permissions: PendingPermission[],
  messages: Message[],
): PendingPermission[] {
  if (current_permissions.length === 0) {
    return current_permissions;
  }

  const completed_round_ids = collectCompletedRoundIds(messages);
  const unresolved_permission_queues = new Map<string, string[]>();
  const loaded_assistant_message_ids = new Set<string>();

  for (const message of messages) {
    if (message.role !== 'assistant') {
      continue;
    }
    loaded_assistant_message_ids.add(message.message_id);

    const resolved_tool_use_ids = new Set<string>();
    for (const block of message.content) {
      if (block.type === 'tool_result') {
        resolved_tool_use_ids.add(block.tool_use_id);
      }
    }

    for (const block of message.content) {
      if (block.type !== 'tool_use' || resolved_tool_use_ids.has(block.id)) {
        continue;
      }

      const signature = buildPermissionSignature(
        block.name,
        block.input as Record<string, unknown>,
      );
      const queue = unresolved_permission_queues.get(signature) ?? [];
      queue.push(message.message_id);
      unresolved_permission_queues.set(signature, queue);
    }
  }

  return current_permissions.filter((permission) => {
    if (permission.caused_by && completed_round_ids.has(permission.caused_by)) {
      return false;
    }

    const signature = buildPermissionSignature(
      permission.tool_name,
      permission.tool_input,
    );
    const queue = unresolved_permission_queues.get(signature);
    if (permission.message_id) {
      const matched_index = queue?.indexOf(permission.message_id) ?? -1;
      if (matched_index < 0) {
        return !loaded_assistant_message_ids.has(permission.message_id);
      }
      queue?.splice(matched_index, 1);
      return true;
    }

    if (!queue?.length) {
      // 中文注释：message_id 缺失时，快照无法证明权限已经结束，
      // 这里优先保留现有卡片，等待后续 replay / result 再收口。
      return true;
    }
    queue.shift();
    return true;
  });
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
  const [pending_agent_slots, set_pending_agent_slots_state] = useState<RoomPendingAgentSlotState[]>([]);
  const [pending_permissions, set_pending_permissions_state] = useState<UseAgentConversationReturn['pending_permissions']>([]);
  const [agent_thinking, set_agent_thinking] = useState<AgentThinkingPayload | null>(null);

  const active_session_key_ref = useRef<string | null>(null);
  const load_request_id_ref = useRef(0);
  const session_seq_cursor_ref = useRef(0);
  const room_seq_cursor_ref = useRef(0);
  const pending_round_ids_ref = useRef<Set<string>>(new Set());
  const active_round_ids_ref = useRef<Set<string>>(new Set());
  const active_message_tracker_ref = useRef<Map<string, ActiveMessageTracker>>(new Map());
  const pending_agent_slots_ref = useRef<RoomPendingAgentSlotState[]>([]);
  const pending_permissions_ref = useRef<UseAgentConversationReturn['pending_permissions']>([]);
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

  const set_pending_agent_slots = useCallback((
    next_state: SetStateAction<RoomPendingAgentSlotState[]>,
  ) => {
    if (typeof next_state !== 'function') {
      pending_agent_slots_ref.current = next_state;
      set_pending_agent_slots_state(next_state);
      return;
    }

    set_pending_agent_slots_state((prev) => {
      const next = next_state(prev);
      pending_agent_slots_ref.current = next;
      return next;
    });
  }, []);

  const set_pending_permissions = useCallback((
    next_state: SetStateAction<UseAgentConversationReturn['pending_permissions']>,
  ) => {
    if (typeof next_state !== 'function') {
      pending_permissions_ref.current = next_state;
      pending_permission_count_ref.current = next_state.length;
      set_pending_permissions_state(next_state);
      return;
    }

    set_pending_permissions_state((prev) => {
      const next = next_state(prev);
      pending_permissions_ref.current = next;
      pending_permission_count_ref.current = next.length;
      return next;
    });
  }, []);

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
      active_round_ids_ref.current.size > 0 ||
      active_message_tracker_ref.current.size > 0 ||
      pending_permission_count_ref.current > 0
    );
    set_is_loading((prev) => (prev === next_is_loading ? prev : next_is_loading));
  }, []);

  const reset_loading_tracker = useCallback(() => {
    pending_round_ids_ref.current.clear();
    active_round_ids_ref.current.clear();
    active_message_tracker_ref.current.clear();
    pending_permission_count_ref.current = 0;
    set_is_loading(false);
  }, []);

  useEffect(() => {
    pending_permission_count_ref.current = pending_permissions.length;
    sync_loading_state();
  }, [pending_permissions.length, sync_loading_state]);

  const reconcile_runtime_state_from_snapshot = useCallback((
    snapshot_messages: Message[],
  ) => {
    const completed_round_ids = collectCompletedRoundIds(snapshot_messages);
    const snapshot_open_round_ids = collectOpenRoundIds(snapshot_messages);
    const terminal_message_ids = new Set<string>();

    for (const message of snapshot_messages) {
      if (message.role !== 'assistant') {
        continue;
      }
      if (message.is_complete || message.stop_reason || isTerminalAssistantStatus(message.stream_status)) {
        terminal_message_ids.add(message.message_id);
      }
    }

    pending_round_ids_ref.current = new Set(
      [...pending_round_ids_ref.current].filter((round_id) => !completed_round_ids.has(round_id)),
    );
    active_round_ids_ref.current = new Set([
      ...[...active_round_ids_ref.current].filter((round_id) => !completed_round_ids.has(round_id)),
      ...snapshot_open_round_ids,
    ]);

    const next_trackers = new Map<string, ActiveMessageTracker>();
    for (const [message_id, tracker] of active_message_tracker_ref.current.entries()) {
      if (completed_round_ids.has(tracker.round_id) || terminal_message_ids.has(message_id)) {
        continue;
      }
      next_trackers.set(message_id, tracker);
    }
    for (const message of snapshot_messages) {
      if (
        message.role !== 'assistant' ||
        message.is_complete ||
        message.stop_reason ||
        isTerminalAssistantStatus(message.stream_status)
      ) {
        continue;
      }
      next_trackers.set(message.message_id, {
        round_id: message.round_id,
        status: message.stream_status ?? 'streaming',
      });
    }
    active_message_tracker_ref.current = next_trackers;

    set_pending_agent_slots(
      filterPendingSlotsFromSnapshot(pending_agent_slots_ref.current, snapshot_messages),
    );
    set_pending_permissions(
      filterPendingPermissionsFromSnapshot(pending_permissions_ref.current, snapshot_messages),
    );
    sync_loading_state();
  }, [set_pending_agent_slots, set_pending_permissions, sync_loading_state]);

  const lifecycle_context: AgentConversationLifecycleContext = useMemo(() => ({
    active_session_key_ref,
    load_request_id_ref,
    agent_id,
    room_id,
    conversation_id,
    chat_type,
    set_session_key,
    set_messages,
    set_pending_agent_slots,
    set_pending_permissions,
    set_is_loading,
    set_error,
    bg_message_cache_ref,
    on_session_messages_loaded: (loaded_messages) => {
      reconcile_runtime_state_from_snapshot(loaded_messages);
    },
  }), [
    active_session_key_ref,
    load_request_id_ref,
    agent_id,
    room_id,
    conversation_id,
    chat_type,
    set_session_key,
    set_messages,
    set_pending_agent_slots,
    set_pending_permissions,
    set_is_loading,
    set_error,
    bg_message_cache_ref,
    reconcile_runtime_state_from_snapshot,
  ]);

  const reload_current_session = useCallback(async () => {
    const active_session_key = active_session_key_ref.current;
    if (!active_session_key) {
      return;
    }

    await loadAgentSession(active_session_key, lifecycle_context, true);
  }, [lifecycle_context]);

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

  const clear_round_tracking = useCallback((
    round_id?: string | null,
    include_related_rounds: boolean = false,
  ) => {
    if (round_id) {
      const should_clear_round = (tracked_round_id: string) => (
        tracked_round_id === round_id ||
        (include_related_rounds && tracked_round_id.startsWith(`${round_id}:`))
      );

      for (const tracked_round_id of [...pending_round_ids_ref.current]) {
        if (should_clear_round(tracked_round_id)) {
          pending_round_ids_ref.current.delete(tracked_round_id);
        }
      }
      for (const tracked_round_id of [...active_round_ids_ref.current]) {
        if (should_clear_round(tracked_round_id)) {
          active_round_ids_ref.current.delete(tracked_round_id);
        }
      }
      for (const [message_id, tracker] of active_message_tracker_ref.current.entries()) {
        if (should_clear_round(tracker.round_id)) {
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
    set_pending_agent_slots((prev) => prev.map((slot) => (
      slot.msg_id === msg_id
        ? {
          ...slot,
          round_id: round_id ?? slot.round_id,
          status,
        }
        : slot
    )));
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
  }, [set_pending_agent_slots, sync_loading_state]);

  const track_chat_ack = useCallback((ack: import('@/types').ChatAckData, _session_key?: string | null) => {
    pending_round_ids_ref.current.delete(ack.round_id);
    const pending_count = ack.pending?.length ?? 0;
    for (const slot of ack.pending ?? []) {
      const agent_round_id = pending_count > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id;
      active_round_ids_ref.current.add(agent_round_id);
      active_message_tracker_ref.current.set(slot.msg_id, {
        round_id: agent_round_id,
        status: 'pending',
      });
    }
    set_pending_agent_slots((prev) => {
      const preserved_slots = prev.filter((slot) => {
        const base_round_id = slot.round_id.split(':', 1)[0];
        return base_round_id !== ack.round_id;
      });
      const next_slots = (ack.pending ?? []).map((slot) => ({
        agent_id: slot.agent_id,
        msg_id: slot.msg_id,
        round_id: pending_count > 1 ? `${ack.round_id}:${slot.agent_id}` : ack.round_id,
        status: 'pending' as const,
        timestamp: Date.now(),
      }));
      return [...preserved_slots, ...next_slots];
    });
    sync_loading_state();
  }, [set_pending_agent_slots, sync_loading_state]);

  const track_assistant_message = useCallback((message: AssistantMessage) => {
    pending_round_ids_ref.current.delete(message.round_id);
    active_round_ids_ref.current.add(message.round_id);

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
    clear_round_tracking(message.round_id);
    set_pending_agent_slots((prev) => prev.filter((slot) => slot.round_id !== message.round_id));
  }, [clear_round_tracking, set_pending_agent_slots]);

  const handle_websocket_message = useCallback((backend_message: unknown) => {
    const event = backend_message as EventMessage;

    if (
      session_key &&
      event.session_key === session_key &&
      typeof event.session_seq === 'number' &&
      event.session_seq > session_seq_cursor_ref.current
    ) {
      session_seq_cursor_ref.current = event.session_seq;
    }

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

    if (
      event.event_type === 'session_resync_required' &&
      event.session_key &&
      is_current_session_event(event.session_key)
    ) {
      const latest_session_seq = event.data?.latest_session_seq;
      if (typeof latest_session_seq === 'number') {
        session_seq_cursor_ref.current = Math.max(
          session_seq_cursor_ref.current,
          latest_session_seq,
        );
      }
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
      set_pending_agent_slots,
      set_pending_permissions,
      enqueue_stream_payload,
      on_background_message,
      set_agent_thinking,
      on_room_event,
      update_message_status,
      clear_round_tracking,
      reset_loading_tracking: reset_loading_tracker,
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
    clear_round_tracking,
    room_id,
    session_key,
    reload_current_session,
    reset_loading_tracker,
    set_pending_agent_slots,
    set_pending_permissions,
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

  useEffect(() => {
    if (ws_state === 'connected') {
      has_connected_ref.current = true;
      set_error(null);
    }
  }, [ws_state]);

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
      ...(session_seq_cursor_ref.current > 0 ? { last_seen_session_seq: session_seq_cursor_ref.current } : {}),
      ...(agent_id ? { agent_id } : {}),
      ...(room_id ? { room_id } : {}),
      ...(conversation_id ? { conversation_id } : {}),
    });
  }, [agent_id, conversation_id, room_id, session_key, ws_send, ws_state]);

  // Subscribe to room-level events (member changes, deletions, etc.) when in a Room context
  useEffect(() => {
    session_seq_cursor_ref.current = 0;
    room_seq_cursor_ref.current = 0;
  }, [room_id, session_key]);

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
    pending_agent_slots,
    messages,
    set_error,
    set_is_loading,
    set_messages,
    set_pending_agent_slots,
    set_pending_permissions,
  }), [agent_id, room_id, conversation_id, chat_type, session_key, ws_state, ws_send, pending_permissions, pending_agent_slots, messages, set_error, set_is_loading, set_messages, set_pending_agent_slots, set_pending_permissions]);

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
      const tracked_round_id = (
        pending_agent_slots_ref.current.find((slot) => slot.msg_id === msg_id)?.round_id
        ?? active_message_tracker_ref.current.get(msg_id)?.round_id
      );
      active_message_tracker_ref.current.delete(msg_id);
      if (tracked_round_id) {
        active_round_ids_ref.current.delete(tracked_round_id);
      }
      set_pending_agent_slots((prev) => prev.map((slot) => (
        slot.msg_id === msg_id
          ? {
            ...slot,
            status: 'cancelled',
          }
          : slot
      )));
      sync_loading_state();
      return;
    }

    const latest_user_round_id = [...messages]
      .reverse()
      .find((message) => message.role === 'user')?.round_id;
    clear_round_tracking(latest_user_round_id, true);
  }, [action_context, clear_round_tracking, messages, set_pending_agent_slots, sync_loading_state]);

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
    const normalized_key = key?.trim() || null;
    if (active_session_key_ref.current === normalized_key) {
      return;
    }

    active_session_key_ref.current = normalized_key;
    set_session_key((current_key) => (
      current_key === normalized_key ? current_key : normalized_key
    ));
    if (!normalized_key) {
      set_pending_agent_slots((current_slots) => (
        current_slots.length ? [] : current_slots
      ));
      set_pending_permissions((current_permissions) => (
        current_permissions.length ? [] : current_permissions
      ));
    }
  }, [set_pending_agent_slots, set_pending_permissions]);

  const reset_session = useCallback(() => {
    resetAgentSession(lifecycle_context);
  }, [lifecycle_context]);

  return {
    error,
    messages,
    session_key,
    ws_state,
    is_loading,
    pending_agent_slots,
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
