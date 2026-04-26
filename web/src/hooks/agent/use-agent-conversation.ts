import {
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  startTransition,
} from "react";
import {
  get_agent_ws_url,
  get_message_history_round_page_size,
  get_message_send_ack_timeout_ms,
} from "@/config/options";
import { get_room_conversation_messages } from "@/lib/api/room-api";
import { are_equivalent_session_keys } from "@/lib/conversation/session-key";
import { get_browser_client_id } from "@/lib/uuid";
import { useWebSocket } from "@/lib/websocket";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import {
  EventMessage,
  Message,
  RoundLifecycleStatus,
  SessionStatusEventPayload,
  StreamMessage,
  WebSocketMessage,
  WebSocketState,
} from "@/types";
import {
  collect_unresolved_tool_use_candidates,
  match_pending_permissions_to_tool_uses,
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import {
  AgentConversationActionContext,
  AgentConversationDeliveryPolicy,
  AgentConversationLifecycleContext,
  AgentConversationSendOptions,
  AgentConversationSessionControlState,
  InputQueueItem,
  RoomEventPayload,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
  get_agent_conversation_identity_key,
} from "@/types/agent/agent-conversation";
import {
  AssistantMessage,
  AssistantMessageStatus,
  RoomPendingAgentSlotState,
} from "@/types";
import { upsert_message } from "./message-helpers";
import {
  clear_agent_session,
  load_agent_session,
  reset_agent_session,
  start_agent_session,
} from "./conversation-lifecycle";
import {
  apply_stream_message,
  dedupe_messages_by_id,
  merge_loaded_messages,
  sort_messages,
} from "./message-helpers";
import { handle_agent_conversation_web_socket_message } from "./websocket-event-handler";
import {
  delete_input_queue_message as send_delete_input_queue_message,
  enqueue_input_queue_message as send_enqueue_input_queue_message,
  guide_input_queue_message as send_guide_input_queue_message,
  reorder_input_queue_messages as send_reorder_input_queue_messages,
  send_session_message,
  send_session_permission_response,
  stop_session_generation,
} from "./conversation-actions";
import {
  AgentConversationRuntimeMachine,
  AgentConversationRuntimeSnapshot,
} from "./agent-conversation-runtime-machine";

interface VolatileConversationSnapshot {
  messages: Message[];
  pending_agent_slots: RoomPendingAgentSlotState[];
  updated_at: number;
}

const VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX =
  "nexus.agent_conversation.volatile";

function is_terminal_slot_status(status: AssistantMessageStatus): boolean {
  return status === "done" || status === "cancelled" || status === "error";
}

function build_volatile_conversation_storage_key(session_key: string): string {
  return `${VOLATILE_CONVERSATION_STORAGE_KEY_PREFIX}:${session_key}`;
}

function get_volatile_conversation_storage(): Storage | null {
  if (typeof window === "undefined") {
    return null;
  }

  try {
    return window.sessionStorage;
  } catch {
    return null;
  }
}

function read_volatile_conversation_snapshot(
  session_key: string,
): VolatileConversationSnapshot | null {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return null;
  }

  try {
    const raw = storage.getItem(
      build_volatile_conversation_storage_key(session_key),
    );
    if (!raw) {
      return null;
    }

    const parsed = JSON.parse(
      raw,
    ) as Partial<VolatileConversationSnapshot> | null;
    if (!parsed) {
      return null;
    }

    return {
      messages: Array.isArray(parsed.messages)
        ? (parsed.messages as Message[])
        : [],
      pending_agent_slots: Array.isArray(parsed.pending_agent_slots)
        ? (parsed.pending_agent_slots as RoomPendingAgentSlotState[])
        : [],
      updated_at: typeof parsed.updated_at === "number" ? parsed.updated_at : 0,
    };
  } catch {
    return null;
  }
}

function write_volatile_conversation_snapshot(
  session_key: string,
  snapshot: VolatileConversationSnapshot,
): void {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return;
  }

  try {
    storage.setItem(
      build_volatile_conversation_storage_key(session_key),
      JSON.stringify(snapshot),
    );
  } catch {
    // sessionStorage 满了时直接忽略，不阻断会话流程
  }
}

function remove_volatile_conversation_snapshot(session_key: string): void {
  const storage = get_volatile_conversation_storage();
  if (!storage) {
    return;
  }

  try {
    storage.removeItem(build_volatile_conversation_storage_key(session_key));
  } catch {
    // 忽略移除失败
  }
}

function merge_pending_agent_slots(
  restored_slots: RoomPendingAgentSlotState[],
  current_slots: RoomPendingAgentSlotState[],
): RoomPendingAgentSlotState[] {
  if (restored_slots.length === 0) {
    return current_slots;
  }

  const merged_slots = new Map<string, RoomPendingAgentSlotState>();
  for (const slot of restored_slots) {
    merged_slots.set(slot.msg_id, slot);
  }
  for (const slot of current_slots) {
    merged_slots.set(slot.msg_id, slot);
  }
  return Array.from(merged_slots.values());
}

function is_ephemeral_message(message: Message): boolean {
  return message.delivery_mode === "ephemeral";
}

function build_volatile_conversation_snapshot(
  messages: Message[],
  runtime_snapshot: AgentConversationRuntimeSnapshot,
  pending_agent_slots: RoomPendingAgentSlotState[],
): VolatileConversationSnapshot | null {
  const active_round_ids = new Set<string>(runtime_snapshot.live_round_ids);

  for (const slot of pending_agent_slots) {
    if (!is_terminal_slot_status(slot.status)) {
      active_round_ids.add(slot.round_id);
    }
  }

  if (active_round_ids.size === 0) {
    return null;
  }

  const volatile_messages = messages.filter((message) => {
    if (is_ephemeral_message(message)) {
      return false;
    }
    if (active_round_ids.has(message.round_id)) {
      return true;
    }

    return (
      message.role === "assistant" &&
      !is_terminal_slot_status(message.stream_status ?? "streaming")
    );
  });
  const volatile_slots = pending_agent_slots.filter(
    (slot) => !is_terminal_slot_status(slot.status),
  );

  if (volatile_messages.length === 0 && volatile_slots.length === 0) {
    return null;
  }

  return {
    messages: volatile_messages,
    pending_agent_slots: volatile_slots,
    updated_at: Date.now(),
  };
}

function filter_pending_slots_from_snapshot(
  current_slots: RoomPendingAgentSlotState[],
  messages: Message[],
  is_round_terminal: (round_id: string) => boolean,
): RoomPendingAgentSlotState[] {
  if (current_slots.length === 0) {
    return current_slots;
  }
  const loaded_message_ids = new Set(
    messages
      .filter(
        (message): message is AssistantMessage => message.role === "assistant",
      )
      .map((message) => message.message_id),
  );

  return current_slots.filter(
    (slot) =>
      !is_round_terminal(slot.round_id) && !loaded_message_ids.has(slot.msg_id),
  );
}

function filter_pending_permissions_from_snapshot(
  current_permissions: PendingPermission[],
  messages: Message[],
  is_round_terminal: (round_id: string) => boolean,
): PendingPermission[] {
  if (current_permissions.length === 0) {
    return current_permissions;
  }
  const loaded_assistant_message_ids = new Set<string>();
  const unresolved_tool_use_candidates =
    collect_unresolved_tool_use_candidates(messages);
  const permission_match_result = match_pending_permissions_to_tool_uses(
    current_permissions,
    unresolved_tool_use_candidates,
  );

  for (const message of messages) {
    if (message.role === "assistant") {
      loaded_assistant_message_ids.add(message.message_id);
    }
  }

  return current_permissions.filter((permission) => {
    if (is_pending_permission_expired(permission)) {
      return false;
    }

    if (permission.caused_by && is_round_terminal(permission.caused_by)) {
      return false;
    }

    if (
      permission_match_result.matched_request_ids.has(permission.request_id)
    ) {
      return true;
    }

    if (!permission.message_id) {
      // 缺少 message_id 的旧权限事件无法做唯一绑定，
      // 快照阶段只能保留，等待明确的 result / reload 收口。
      return true;
    }

    return !loaded_assistant_message_ids.has(permission.message_id);
  });
}

function get_pending_permission_expiration_ms(
  permission: PendingPermission,
): number | null {
  if (!permission.expires_at) {
    return null;
  }
  const expires_at_ms = Date.parse(permission.expires_at);
  return Number.isFinite(expires_at_ms) ? expires_at_ms : null;
}

function is_pending_permission_expired(
  permission: PendingPermission,
  now_ms: number = Date.now(),
): boolean {
  const expires_at_ms = get_pending_permission_expiration_ms(permission);
  return expires_at_ms != null && expires_at_ms <= now_ms;
}

function prune_expired_pending_permissions(
  current_permissions: PendingPermission[],
  now_ms: number = Date.now(),
): PendingPermission[] {
  if (current_permissions.length === 0) {
    return current_permissions;
  }

  const next_permissions = current_permissions.filter(
    (permission) => !is_pending_permission_expired(permission, now_ms),
  );
  return next_permissions.length === current_permissions.length
    ? current_permissions
    : next_permissions;
}

function get_next_pending_permission_timeout_ms(
  current_permissions: PendingPermission[],
  now_ms: number = Date.now(),
): number | null {
  let next_timeout_ms: number | null = null;

  for (const permission of current_permissions) {
    const expires_at_ms = get_pending_permission_expiration_ms(permission);
    if (expires_at_ms == null) {
      continue;
    }
    const timeout_ms = Math.max(expires_at_ms - now_ms, 0);
    if (next_timeout_ms == null || timeout_ms < next_timeout_ms) {
      next_timeout_ms = timeout_ms;
    }
  }

  return next_timeout_ms;
}

function are_runtime_snapshots_equal(
  left: AgentConversationRuntimeSnapshot,
  right: AgentConversationRuntimeSnapshot,
): boolean {
  if (
    left.phase !== right.phase ||
    left.pending_permission_count !== right.pending_permission_count ||
    left.is_loading !== right.is_loading
  ) {
    return false;
  }

  const are_string_arrays_equal = (lhs: string[], rhs: string[]): boolean => {
    if (lhs.length !== rhs.length) {
      return false;
    }

    for (let index = 0; index < lhs.length; index += 1) {
      if (lhs[index] !== rhs[index]) {
        return false;
      }
    }
    return true;
  };

  if (
    !are_string_arrays_equal(left.sending_round_ids, right.sending_round_ids) ||
    !are_string_arrays_equal(left.running_round_ids, right.running_round_ids) ||
    !are_string_arrays_equal(
      left.terminal_round_ids,
      right.terminal_round_ids,
    ) ||
    !are_string_arrays_equal(left.live_round_ids, right.live_round_ids)
  ) {
    return false;
  }

  const left_message_ids = Object.keys(left.active_messages);
  const right_message_ids = Object.keys(right.active_messages);
  if (!are_string_arrays_equal(left_message_ids, right_message_ids)) {
    return false;
  }

  for (const message_id of left_message_ids) {
    const left_tracker = left.active_messages[message_id];
    const right_tracker = right.active_messages[message_id];
    if (
      !right_tracker ||
      left_tracker.round_id !== right_tracker.round_id ||
      left_tracker.status !== right_tracker.status
    ) {
      return false;
    }
  }

  return true;
}

function matches_round_lifecycle(
  round_id: string,
  target_round_id: string,
): boolean {
  return (
    round_id === target_round_id || round_id.startsWith(`${target_round_id}:`)
  );
}

function get_terminal_message_status(
  status: RoundLifecycleStatus,
): AssistantMessageStatus {
  if (status === "interrupted") {
    return "cancelled";
  }
  if (status === "error") {
    return "error";
  }
  return "done";
}

export function useAgentConversation(
  options: UseAgentConversationOptions = {},
): UseAgentConversationReturn {
  const ws_url = options.ws_url || get_agent_ws_url();
  const identity = options.identity ?? null;
  const agent_id = identity?.agent_id ?? null;
  const room_id = identity?.room_id ?? null;
  const conversation_id = identity?.conversation_id ?? null;
  const chat_type = identity?.chat_type ?? "dm";
  const on_error = options.on_error;
  const on_room_event_callback = options.on_room_event;
  const apply_workspace_event = useWorkspaceLiveStore(
    (state) => state.apply_event,
  );
  const runtime_machine_ref = useRef(
    new AgentConversationRuntimeMachine(chat_type),
  );
  const [runtime_snapshot, set_runtime_snapshot] =
    useState<AgentConversationRuntimeSnapshot>(() =>
      runtime_machine_ref.current.snapshot(),
    );

  const [messages, set_messages_state] = useState<Message[]>([]);
  const [error, set_error] = useState<string | null>(null);
  const [session_key, set_session_key] = useState<string | null>(
    identity?.session_key ?? null,
  );
  const [is_session_loading, set_is_session_loading] = useState(false);
  const [is_history_loading, set_is_history_loading_state] = useState(false);
  const [has_more_history, set_has_more_history_state] = useState(false);
  const [history_prepend_token, set_history_prepend_token] = useState(0);
  const [pending_agent_slots, set_pending_agent_slots_state] = useState<
    RoomPendingAgentSlotState[]
  >([]);
  const [input_queue_items, set_input_queue_items_state] = useState<
    InputQueueItem[]
  >([]);
  const [pending_permissions, set_pending_permissions_state] = useState<
    UseAgentConversationReturn["pending_permissions"]
  >([]);
  const [session_control_state, set_session_control_state] =
    useState<AgentConversationSessionControlState>("unknown");
  const [session_controller_client_id, set_session_controller_client_id] =
    useState<string | null>(null);
  const [session_observer_count, set_session_observer_count] = useState(0);

  const active_session_key_ref = useRef<string | null>(
    identity?.session_key ?? null,
  );
  const active_identity_key_ref = useRef<string | null>(
    get_agent_conversation_identity_key(identity),
  );
  const browser_client_id_ref = useRef<string>(get_browser_client_id());
  const load_request_id_ref = useRef(0);
  const session_seq_cursor_ref = useRef(0);
  const room_seq_cursor_ref = useRef(0);
  const is_history_loading_ref = useRef(false);
  const has_more_history_ref = useRef(false);
  const history_cursor_ref = useRef<{
    before_round_id: string | null;
    before_round_timestamp: number | null;
  }>({
    before_round_id: null,
    before_round_timestamp: null,
  });
  const pending_agent_slots_ref = useRef<RoomPendingAgentSlotState[]>([]);
  const pending_permissions_ref = useRef<
    UseAgentConversationReturn["pending_permissions"]
  >([]);
  const pending_chat_ack_ref = useRef<
    Map<
      string,
      {
        reject: (error: Error) => void;
        resolve: () => void;
        timeout_id: number;
      }
    >
  >(new Map());
  const ws_send_ref = useRef<
    (payload: WebSocketMessage) => {
      disposition: "sent" | "queued" | "dropped";
    }
  >(() => ({ disposition: "dropped" }));
  const ws_reconnect_ref = useRef<() => void>(() => {});
  const ws_state_ref = useRef<WebSocketState>("disconnected");
  // Per-session message cache: accumulates messages received for non-active sessions
  // so they are not lost when the user switches conversations.
  const bg_message_cache_ref = useRef<Map<string, Message[]>>(new Map());
  const is_loading = runtime_snapshot.is_loading;
  const runtime_phase = runtime_snapshot.phase;
  const live_round_ids = runtime_snapshot.live_round_ids;
  const is_session_controller = session_control_state === "controller";

  const set_messages = useCallback((next_state: SetStateAction<Message[]>) => {
    set_messages_state((current_messages) => {
      const next_messages =
        typeof next_state === "function"
          ? next_state(current_messages)
          : next_state;
      return dedupe_messages_by_id(next_messages);
    });
  }, []);

  const set_history_loading = useCallback((next_value: boolean) => {
    is_history_loading_ref.current = next_value;
    set_is_history_loading_state((current_value) =>
      current_value === next_value ? current_value : next_value,
    );
  }, []);

  const set_has_more_history = useCallback((next_value: boolean) => {
    has_more_history_ref.current = next_value;
    set_has_more_history_state((current_value) =>
      current_value === next_value ? current_value : next_value,
    );
  }, []);

  const reset_history_state = useCallback(() => {
    history_cursor_ref.current = {
      before_round_id: null,
      before_round_timestamp: null,
    };
    set_history_loading(false);
    set_has_more_history(false);
  }, [set_has_more_history, set_history_loading]);

  // ── Stream batching ──────────────────────────────────────────────────────
  // WebSocket fires on every token (~50-100/sec during streaming).
  // Each token previously called set_messages + set_is_loading = 2 React renders.
  // At 100 tokens/sec that's 200 renders/sec and full CPU saturation.
  //
  // Fix: accumulate stream payloads within an animation frame, flush them all
  // in one setState per frame (≤60 flushes/sec regardless of token rate).
  // startTransition marks the update as non-urgent so React can interrupt it
  // if a higher-priority update (e.g. user keypress) arrives.
  const stream_buffer_ref = useRef<StreamMessage[]>([]);
  const stream_raf_ref = useRef<number | null>(null);

  const sync_runtime_snapshot = useCallback(() => {
    const next_snapshot = runtime_machine_ref.current.snapshot();
    set_runtime_snapshot((current_snapshot) =>
      are_runtime_snapshots_equal(current_snapshot, next_snapshot)
        ? current_snapshot
        : next_snapshot,
    );
  }, []);

  const apply_runtime_transition = useCallback(
    (transition: (machine: AgentConversationRuntimeMachine) => void) => {
      transition(runtime_machine_ref.current);
      sync_runtime_snapshot();
    },
    [sync_runtime_snapshot],
  );

  const set_pending_agent_slots = useCallback(
    (next_state: SetStateAction<RoomPendingAgentSlotState[]>) => {
      const next =
        typeof next_state === "function"
          ? next_state(pending_agent_slots_ref.current)
          : next_state;
      pending_agent_slots_ref.current = next;
      set_pending_agent_slots_state(next);
    },
    [],
  );

  const set_input_queue_items = useCallback(
    (next_state: SetStateAction<InputQueueItem[]>) => {
      set_input_queue_items_state((current_items) =>
        typeof next_state === "function"
          ? next_state(current_items)
          : next_state,
      );
    },
    [],
  );

  const set_pending_permissions = useCallback(
    (
      next_state: SetStateAction<
        UseAgentConversationReturn["pending_permissions"]
      >,
    ) => {
      const next =
        typeof next_state === "function"
          ? next_state(pending_permissions_ref.current)
          : next_state;
      pending_permissions_ref.current = next;
      apply_runtime_transition((machine) => {
        machine.set_pending_permission_count(next.length);
      });
      set_pending_permissions_state(next);
    },
    [apply_runtime_transition],
  );

  const is_current_session_event = useCallback(
    (incoming_session_key?: string | null) => {
      if (!incoming_session_key) {
        return false;
      }
      return are_equivalent_session_keys(
        active_session_key_ref.current,
        incoming_session_key,
      );
    },
    [],
  );

  const on_background_message = useCallback((key: string, message: Message) => {
    if (is_ephemeral_message(message)) {
      return;
    }
    const cache = bg_message_cache_ref.current;
    const existing = cache.get(key) ?? [];
    const next = upsert_message(existing, message);
    cache.set(key, next);
  }, []);

  const on_room_event = useCallback(
    (event_type: string, data: RoomEventPayload) => {
      on_room_event_callback?.(event_type, data);
    },
    [on_room_event_callback],
  );

  const clear_pending_chat_ack = useCallback((round_id?: string | null) => {
    if (!round_id) {
      return false;
    }
    const pending_request = pending_chat_ack_ref.current.get(round_id);
    if (!pending_request) {
      return false;
    }
    window.clearTimeout(pending_request.timeout_id);
    pending_chat_ack_ref.current.delete(round_id);
    pending_request.resolve();
    return true;
  }, []);

  const cancel_pending_chat_acks = useCallback((reason: string) => {
    for (const [
      round_id,
      pending_request,
    ] of pending_chat_ack_ref.current.entries()) {
      window.clearTimeout(pending_request.timeout_id);
      pending_request.reject(new Error(reason));
      pending_chat_ack_ref.current.delete(round_id);
    }
  }, []);

  const fail_pending_chat_ack = useCallback(
    (round_id: string, message: string) => {
      const pending_request = pending_chat_ack_ref.current.get(round_id);
      if (!pending_request) {
        return;
      }
      window.clearTimeout(pending_request.timeout_id);
      pending_chat_ack_ref.current.delete(round_id);
      pending_request.reject(new Error(message));
      apply_runtime_transition((machine) => {
        machine.clear_round(round_id, chat_type === "group");
      });
      set_pending_agent_slots((prev) =>
        prev.filter(
          (slot) => !matches_round_lifecycle(slot.round_id, round_id),
        ),
      );
      set_pending_permissions((prev) =>
        prev.filter(
          (permission) =>
            !permission.caused_by ||
            !matches_round_lifecycle(permission.caused_by, round_id),
        ),
      );
      set_messages((prev) =>
        prev.filter(
          (message) =>
            !(
              message.role === "user" &&
              message.message_id === round_id &&
              message.round_id === round_id
            ),
        ),
      );
      set_error(message);
      if (ws_state_ref.current === "connected") {
        ws_reconnect_ref.current();
      }
    },
    [
      apply_runtime_transition,
      chat_type,
      set_messages,
      set_pending_agent_slots,
      set_pending_permissions,
    ],
  );

  const reset_session_control = useCallback(() => {
    set_session_control_state("unknown");
    set_session_controller_client_id(null);
    set_session_observer_count(0);
  }, []);

  const reset_runtime_machine = useCallback(() => {
    apply_runtime_transition((machine) => {
      machine.reset();
    });
  }, [apply_runtime_transition]);

  const reconcile_runtime_state_from_snapshot = useCallback(
    (snapshot_messages: Message[]) => {
      apply_runtime_transition((machine) => {
        machine.reconcile_from_snapshot(snapshot_messages);
      });
      const is_round_terminal = (round_id: string) =>
        runtime_machine_ref.current.is_round_terminal(round_id);

      set_pending_agent_slots(
        filter_pending_slots_from_snapshot(
          pending_agent_slots_ref.current,
          snapshot_messages,
          is_round_terminal,
        ),
      );
      set_pending_permissions(
        filter_pending_permissions_from_snapshot(
          pending_permissions_ref.current,
          snapshot_messages,
          is_round_terminal,
        ),
      );
    },
    [
      apply_runtime_transition,
      set_pending_agent_slots,
      set_pending_permissions,
    ],
  );

  const lifecycle_context: AgentConversationLifecycleContext = useMemo(
    () => ({
      active_session_key_ref,
      load_request_id_ref,
      identity,
      set_session_key,
      set_is_session_loading,
      set_messages,
      set_pending_agent_slots,
      set_input_queue_items,
      set_pending_permissions,
      set_error,
      bg_message_cache_ref,
      restore_volatile_session_snapshot: (target_session_key) => {
        const snapshot =
          read_volatile_conversation_snapshot(target_session_key);
        if (!snapshot) {
          return false;
        }

        let restored_messages = snapshot.messages;
        set_messages((current_messages) => {
          restored_messages = merge_loaded_messages(
            snapshot.messages,
            current_messages,
          );
          return restored_messages;
        });
        set_pending_agent_slots((current_slots) =>
          merge_pending_agent_slots(
            snapshot.pending_agent_slots,
            current_slots,
          ),
        );
        set_error(null);
        reconcile_runtime_state_from_snapshot(restored_messages);
        return (
          restored_messages.length > 0 ||
          snapshot.pending_agent_slots.length > 0
        );
      },
      on_session_messages_loaded: (loaded_messages, meta) => {
        if (!meta.is_reload) {
          history_cursor_ref.current = {
            before_round_id: meta.next_before_round_id,
            before_round_timestamp: meta.next_before_round_timestamp,
          };
          set_has_more_history(meta.has_more_history);
        }
        reconcile_runtime_state_from_snapshot(loaded_messages);
      },
    }),
    [
      active_session_key_ref,
      load_request_id_ref,
      identity,
      set_session_key,
      set_is_session_loading,
      set_messages,
      set_pending_agent_slots,
      set_input_queue_items,
      set_pending_permissions,
      set_error,
      bg_message_cache_ref,
      reconcile_runtime_state_from_snapshot,
      set_has_more_history,
    ],
  );

  useEffect(() => {
    if (!session_key) {
      return;
    }

    const snapshot = build_volatile_conversation_snapshot(
      messages,
      runtime_snapshot,
      pending_agent_slots,
    );
    if (!snapshot) {
      remove_volatile_conversation_snapshot(session_key);
      return;
    }

    write_volatile_conversation_snapshot(session_key, snapshot);
  }, [messages, pending_agent_slots, runtime_snapshot, session_key]);

  useEffect(() => {
    const next_permissions = prune_expired_pending_permissions(
      pending_permissions_ref.current,
    );
    if (next_permissions !== pending_permissions_ref.current) {
      set_pending_permissions(next_permissions);
      return;
    }

    const next_timeout_ms = get_next_pending_permission_timeout_ms(
      pending_permissions_ref.current,
    );
    if (next_timeout_ms == null) {
      return;
    }

    const timeout_id = window.setTimeout(() => {
      set_pending_permissions((current_permissions) =>
        prune_expired_pending_permissions(current_permissions),
      );
    }, next_timeout_ms + 1);

    return () => {
      window.clearTimeout(timeout_id);
    };
  }, [pending_permissions, set_pending_permissions]);

  const reload_current_session = useCallback(async () => {
    const active_session_key = active_session_key_ref.current;
    if (!active_session_key) {
      return;
    }

    await load_agent_session(active_session_key, lifecycle_context, true);
  }, [lifecycle_context]);

  const load_older_messages = useCallback(async (): Promise<boolean> => {
    const active_session_key = active_session_key_ref.current;
    const current_room_id = identity?.room_id?.trim() ?? "";
    const current_conversation_id = identity?.conversation_id?.trim() ?? "";
    const before_round_id = history_cursor_ref.current.before_round_id;
    const before_round_timestamp =
      history_cursor_ref.current.before_round_timestamp;

    if (
      !active_session_key ||
      !current_room_id ||
      !current_conversation_id ||
      !has_more_history_ref.current ||
      is_history_loading_ref.current ||
      !before_round_timestamp
    ) {
      return false;
    }

    set_history_loading(true);
    try {
      const page = await get_room_conversation_messages(
        current_room_id,
        current_conversation_id,
        {
          limit: get_message_history_round_page_size(),
          before_round_id,
          before_round_timestamp,
        },
      );
      if (active_session_key_ref.current !== active_session_key) {
        return false;
      }

      const sorted_messages = sort_messages(page.items ?? []);
      if (sorted_messages.length === 0) {
        history_cursor_ref.current = {
          before_round_id: null,
          before_round_timestamp: null,
        };
        set_has_more_history(false);
        return false;
      }

      set_messages((current_messages) =>
        merge_loaded_messages(sorted_messages, current_messages),
      );
      history_cursor_ref.current = {
        before_round_id: page.next_before_round_id ?? null,
        before_round_timestamp: page.next_before_round_timestamp ?? null,
      };
      set_has_more_history(page.has_more ?? false);
      set_history_prepend_token((current_token) => current_token + 1);
      return true;
    } catch (err) {
      if (active_session_key_ref.current !== active_session_key) {
        return false;
      }
      console.error("[useAgentConversation] 加载更早消息失败:", err);
      set_error(
        err instanceof Error ? err.message : "Failed to load older messages",
      );
      return false;
    } finally {
      if (active_session_key_ref.current === active_session_key) {
        set_history_loading(false);
      }
    }
  }, [
    identity?.conversation_id,
    identity?.room_id,
    set_error,
    set_has_more_history,
    set_history_loading,
    set_messages,
  ]);

  const flush_stream_buffer = useCallback(() => {
    stream_raf_ref.current = null;
    const payloads = stream_buffer_ref.current;
    if (payloads.length === 0) return;
    stream_buffer_ref.current = [];

    startTransition(() => {
      set_messages((prev) => {
        let next = prev;
        for (const payload of payloads) {
          next = apply_stream_message(next, payload);
        }
        return next;
      });
    });
  }, [set_messages]);

  const enqueue_stream_payload = useCallback(
    (payload: StreamMessage) => {
      stream_buffer_ref.current.push(payload);
      if (stream_raf_ref.current === null) {
        stream_raf_ref.current = requestAnimationFrame(flush_stream_buffer);
      }
    },
    [flush_stream_buffer],
  );

  const reconcile_stopped_session = useCallback(() => {
    const runtime_snapshot_before_reset =
      runtime_machine_ref.current.snapshot();
    const terminal_round_ids = new Set(
      runtime_snapshot_before_reset.terminal_round_ids,
    );
    const is_terminal_round = (round_id: string) => {
      if (terminal_round_ids.has(round_id)) {
        return true;
      }
      if (chat_type !== "group") {
        return false;
      }
      for (const terminal_round_id of terminal_round_ids) {
        if (round_id.startsWith(`${terminal_round_id}:`)) {
          return true;
        }
      }
      return false;
    };
    apply_runtime_transition((machine) => {
      machine.reset();
    });
    set_pending_permissions([]);
    set_pending_agent_slots((prev) =>
      prev.map((slot) =>
        slot.status === "cancelled" || slot.status === "error"
          ? slot
          : {
              ...slot,
              status: "cancelled",
            },
      ),
    );
    set_messages((prev) => {
      let has_changes = false;
      const next_messages: Message[] = [];
      for (const message of prev) {
        if (is_ephemeral_message(message)) {
          has_changes = true;
          continue;
        }
        if (message.role !== "assistant") {
          next_messages.push(message);
          continue;
        }
        if (is_terminal_round(message.round_id)) {
          next_messages.push(message);
          continue;
        }
        if (
          message.stop_reason ||
          message.stream_status === "done" ||
          message.stream_status === "cancelled" ||
          message.stream_status === "error"
        ) {
          next_messages.push(message);
          continue;
        }
        has_changes = true;
        next_messages.push({
          ...message,
          stream_status: "cancelled" as const,
        });
      }
      return has_changes ? next_messages : prev;
    });
  }, [
    apply_runtime_transition,
    chat_type,
    set_messages,
    set_pending_agent_slots,
    set_pending_permissions,
  ]);

  const sync_session_status = useCallback(
    (payload: SessionStatusEventPayload) => {
      const next_controller_client_id =
        typeof payload.controller_client_id === "string" &&
        payload.controller_client_id
          ? payload.controller_client_id
          : null;
      const next_observer_count =
        typeof payload.observer_count === "number" &&
        payload.observer_count >= 0
          ? payload.observer_count
          : 0;
      set_session_controller_client_id((current_controller_client_id) =>
        current_controller_client_id === next_controller_client_id
          ? current_controller_client_id
          : next_controller_client_id,
      );
      set_session_observer_count((current_observer_count) =>
        current_observer_count === next_observer_count
          ? current_observer_count
          : next_observer_count,
      );
      set_session_control_state((current_state) => {
        const next_state: AgentConversationSessionControlState =
          !next_controller_client_id
            ? "unknown"
            : next_controller_client_id === browser_client_id_ref.current
              ? "controller"
              : "observer";
        return current_state === next_state ? current_state : next_state;
      });

      const running_round_ids = Array.isArray(payload.running_round_ids)
        ? payload.running_round_ids.filter(
            (round_id): round_id is string => typeof round_id === "string",
          )
        : [];
      if (!payload.is_generating || running_round_ids.length === 0) {
        reconcile_stopped_session();
        return;
      }
      apply_runtime_transition((machine) => {
        machine.sync_running_rounds(running_round_ids);
      });
    },
    [apply_runtime_transition, reconcile_stopped_session],
  );

  const update_message_status = useCallback(
    (
      msg_id: string,
      status: AssistantMessageStatus,
      round_id?: string | null,
    ) => {
      set_messages((prev) =>
        prev.map((m) =>
          m.message_id === msg_id && m.role === "assistant"
            ? { ...(m as AssistantMessage), stream_status: status }
            : m,
        ),
      );
      set_pending_agent_slots((prev) =>
        prev.map((slot) =>
          slot.msg_id === msg_id
            ? {
                ...slot,
                round_id: round_id ?? slot.round_id,
                status,
              }
            : slot,
        ),
      );
      apply_runtime_transition((machine) => {
        machine.update_message_status(msg_id, status, round_id);
      });
    },
    [apply_runtime_transition, set_messages, set_pending_agent_slots],
  );

  const track_chat_ack = useCallback(
    (ack: import("@/types").ChatAckData, _session_key?: string | null) => {
      apply_runtime_transition((machine) => {
        machine.track_chat_ack(ack);
      });
      clear_pending_chat_ack(ack.round_id);
      const pending_count = ack.pending?.length ?? 0;
      set_pending_agent_slots((prev) => {
        const preserved_slots = prev.filter((slot) => {
          const base_round_id = slot.round_id.split(":", 1)[0];
          return base_round_id !== ack.round_id;
        });
        const next_slots = (ack.pending ?? []).map((slot) => ({
          agent_id: slot.agent_id,
          msg_id: slot.msg_id,
          round_id:
            slot.round_id ||
            (pending_count > 1
              ? `${ack.round_id}:${slot.agent_id}`
              : ack.round_id),
          status: (slot.status ?? "pending") as AssistantMessageStatus,
          timestamp: slot.timestamp ?? Date.now(),
        }));
        return [...preserved_slots, ...next_slots];
      });
    },
    [apply_runtime_transition, clear_pending_chat_ack, set_pending_agent_slots],
  );

  const track_assistant_message = useCallback(
    (message: AssistantMessage) => {
      clear_pending_chat_ack(message.round_id);
      apply_runtime_transition((machine) => {
        machine.track_assistant_message(message);
      });
    },
    [apply_runtime_transition, clear_pending_chat_ack],
  );

  const apply_round_status = useCallback(
    (round_id: string, status: RoundLifecycleStatus) => {
      apply_runtime_transition((machine) => {
        machine.track_round_status(round_id, status);
      });
      clear_pending_chat_ack(round_id);

      if (status === "running") {
        return;
      }

      const terminal_status = get_terminal_message_status(status);
      set_pending_permissions((prev) =>
        prev.filter((permission) => {
          if (!permission.caused_by) {
            return true;
          }
          return !matches_round_lifecycle(permission.caused_by, round_id);
        }),
      );
      set_pending_agent_slots((prev) =>
        prev.filter(
          (slot) => !matches_round_lifecycle(slot.round_id, round_id),
        ),
      );
      set_messages((prev) => {
        let has_changes = false;
        const next_messages: Message[] = [];
        for (const message of prev) {
          if (
            matches_round_lifecycle(message.round_id, round_id) &&
            is_ephemeral_message(message)
          ) {
            has_changes = true;
            continue;
          }
          if (message.role !== "assistant") {
            next_messages.push(message);
            continue;
          }
          if (!matches_round_lifecycle(message.round_id, round_id)) {
            next_messages.push(message);
            continue;
          }
          if (
            message.stream_status === terminal_status ||
            message.stream_status === "cancelled" ||
            message.stream_status === "error" ||
            message.stream_status === "done"
          ) {
            next_messages.push(message);
            continue;
          }
          has_changes = true;
          next_messages.push({
            ...message,
            stream_status: terminal_status,
          });
        }
        return has_changes ? next_messages : prev;
      });
    },
    [
      apply_runtime_transition,
      clear_pending_chat_ack,
      set_messages,
      set_pending_agent_slots,
      set_pending_permissions,
    ],
  );

  const build_session_bind_message = useCallback(
    (target_session_key: string): WebSocketMessage => ({
      type: "bind_session",
      session_key: target_session_key,
      client_id: browser_client_id_ref.current,
      // 自动重绑只恢复观察关系，不主动抢占控制权。
      // 这样多窗口之间不会因为聚焦或重连把主理人被动抢走。
      request_control: false,
      ...(session_seq_cursor_ref.current > 0
        ? { last_seen_session_seq: session_seq_cursor_ref.current }
        : {}),
      ...(agent_id ? { agent_id } : {}),
      ...(room_id ? { room_id } : {}),
      ...(conversation_id ? { conversation_id } : {}),
    }),
    [agent_id, conversation_id, room_id],
  );

  const handle_websocket_message = useCallback(
    (backend_message: unknown) => {
      const event = backend_message as EventMessage;

      if (
        session_key &&
        event.session_key === session_key &&
        typeof event.session_seq === "number" &&
        event.session_seq > session_seq_cursor_ref.current
      ) {
        session_seq_cursor_ref.current = event.session_seq;
      }

      if (
        room_id &&
        event.room_id === room_id &&
        typeof event.room_seq === "number" &&
        event.room_seq > room_seq_cursor_ref.current
      ) {
        room_seq_cursor_ref.current = event.room_seq;
      }

      if (
        event.event_type === "room_resync_required" &&
        event.room_id === room_id
      ) {
        const latest_room_seq = event.data?.latest_room_seq;
        if (typeof latest_room_seq === "number") {
          room_seq_cursor_ref.current = Math.max(
            room_seq_cursor_ref.current,
            latest_room_seq,
          );
        }
        on_room_event_callback?.(event.event_type, event.data ?? {});
        void reload_current_session().finally(() => {
          if (!room_id || ws_state_ref.current !== "connected") {
            return;
          }
          ws_send_ref.current({
            type: "subscribe_room",
            room_id,
            conversation_id,
            ...(room_seq_cursor_ref.current > 0
              ? { last_seen_room_seq: room_seq_cursor_ref.current }
              : {}),
          });
        });
        return;
      }

      if (
        event.event_type === "session_resync_required" &&
        event.session_key &&
        is_current_session_event(event.session_key)
      ) {
        const latest_session_seq = event.data?.latest_session_seq;
        if (typeof latest_session_seq === "number") {
          session_seq_cursor_ref.current = Math.max(
            session_seq_cursor_ref.current,
            latest_session_seq,
          );
        }
        void reload_current_session().finally(() => {
          if (!session_key || ws_state_ref.current !== "connected") {
            return;
          }
          ws_send_ref.current(build_session_bind_message(session_key));
        });
        return;
      }

      handle_agent_conversation_web_socket_message({
        backend_message,
        apply_workspace_event,
        is_current_session_event,
        set_error,
        set_messages,
        set_pending_agent_slots,
        set_input_queue_items,
        set_pending_permissions,
        enqueue_stream_payload,
        on_background_message,
        on_room_event,
        update_message_status,
        sync_session_status,
        apply_round_status,
        track_chat_ack,
        track_assistant_message,
      });
    },
    [
      apply_workspace_event,
      is_current_session_event,
      enqueue_stream_payload,
      on_background_message,
      on_room_event,
      on_room_event_callback,
      room_id,
      session_key,
      conversation_id,
      build_session_bind_message,
      reload_current_session,
      apply_round_status,
      set_pending_agent_slots,
      set_input_queue_items,
      set_messages,
      set_pending_permissions,
      sync_session_status,
      track_assistant_message,
      track_chat_ack,
      update_message_status,
    ],
  );

  useEffect(() => {
    runtime_machine_ref.current.set_chat_type(chat_type);
    sync_runtime_snapshot();
  }, [chat_type, sync_runtime_snapshot]);

  useEffect(() => {
    const next_identity_key = get_agent_conversation_identity_key(identity);
    if (active_identity_key_ref.current === next_identity_key) {
      return;
    }

    active_identity_key_ref.current = next_identity_key;
    cancel_pending_chat_acks("会话上下文已切换，未确认的消息发送已取消");
    session_seq_cursor_ref.current = 0;
    room_seq_cursor_ref.current = 0;
    reset_runtime_machine();
    reset_session_control();
    reset_history_state();
    set_history_prepend_token(0);
    set_pending_agent_slots((current_slots) =>
      current_slots.length ? [] : current_slots,
    );
    set_input_queue_items((current_items) =>
      current_items.length ? [] : current_items,
    );
    set_pending_permissions((current_permissions) =>
      current_permissions.length ? [] : current_permissions,
    );
  }, [
    cancel_pending_chat_acks,
    identity,
    reset_history_state,
    reset_runtime_machine,
    reset_session_control,
    set_pending_agent_slots,
    set_input_queue_items,
    set_pending_permissions,
  ]);

  useEffect(() => {
    const next_session_key = identity?.session_key?.trim() || null;
    active_session_key_ref.current = next_session_key;
    set_session_key((current_session_key) =>
      current_session_key === next_session_key
        ? current_session_key
        : next_session_key,
    );
  }, [identity?.session_key]);

  // Cancel any pending rAF flush on unmount to prevent setState after unmount
  useEffect(() => {
    return () => {
      cancel_pending_chat_acks("会话已卸载，未确认的消息发送已取消");
      if (stream_raf_ref.current !== null) {
        cancelAnimationFrame(stream_raf_ref.current);
        stream_raf_ref.current = null;
      }
    };
  }, [cancel_pending_chat_acks]);

  const has_connected_ref = useRef(false);

  const {
    state: ws_state,
    send: ws_send,
    reconnect: ws_reconnect,
  } = useWebSocket({
    url: ws_url,
    auto_connect: true,
    reconnect: true,
    heartbeat_interval: 30000,
    on_message: handle_websocket_message,
    on_error: (event) => {
      // 开发环境 StrictMode 会触发一次挂载后立即清理，
      // 这时 connecting 阶段被主动断开会产生一次无意义的 error。
      if (!has_connected_ref.current) {
        console.debug(
          "[useAgentConversation] Ignored transient WebSocket error before first successful connection",
          event,
        );
        return;
      }

      const error_message = "WebSocket error occurred";
      console.error("[useAgentConversation] WebSocket error:", event);
      set_error(error_message);
      on_error?.(new Error(error_message));
    },
  });

  useEffect(() => {
    ws_send_ref.current = ws_send;
  }, [ws_send]);

  useEffect(() => {
    ws_reconnect_ref.current = ws_reconnect;
  }, [ws_reconnect]);

  useEffect(() => {
    ws_state_ref.current = ws_state;
  }, [ws_state]);

  useEffect(() => {
    if (ws_state === "connected") {
      has_connected_ref.current = true;
      set_error(null);
    }
  }, [ws_state]);

  useEffect(() => {
    if (!agent_id || ws_state !== "connected") {
      return;
    }

    ws_send({
      type: "subscribe_workspace",
      agent_id,
    });

    return () => {
      ws_send({
        type: "unsubscribe_workspace",
        agent_id,
      });
    };
  }, [agent_id, ws_send, ws_state]);

  useEffect(() => {
    if (!session_key || ws_state !== "connected") {
      return;
    }
    const client_id = browser_client_id_ref.current;

    // WebSocket 重连后，后端需要重新知道“当前这个连接服务哪个 session”，
    // 否则挂起中的权限请求无法重投到新连接。
    ws_send(build_session_bind_message(session_key));

    return () => {
      // 共享 WebSocket 常驻于应用路由壳后，
      // 会话组件卸载时必须显式解绑旧 session，避免权限请求和 session 状态继续路由到已离开的页面上下文。
      ws_send({
        type: "unbind_session",
        session_key,
        client_id,
      });
    };
  }, [build_session_bind_message, session_key, ws_send, ws_state]);

  // Subscribe to room-level events (member changes, deletions, etc.) when in a Room context
  useEffect(() => {
    session_seq_cursor_ref.current = 0;
    room_seq_cursor_ref.current = 0;
  }, [room_id, session_key]);

  useEffect(() => {
    if (!room_id || ws_state !== "connected") {
      return;
    }

    ws_send({
      type: "subscribe_room",
      room_id,
      conversation_id,
      ...(room_seq_cursor_ref.current > 0
        ? { last_seen_room_seq: room_seq_cursor_ref.current }
        : {}),
    });

    return () => {
      ws_send({
        type: "unsubscribe_room",
        room_id,
        conversation_id,
      });
    };
  }, [conversation_id, room_id, ws_send, ws_state]);

  const action_context: AgentConversationActionContext = useMemo(
    () => ({
      identity,
      session_key,
      ws_state,
      session_control_state,
      ws_send,
      active_session_key_ref,
      pending_permissions,
      pending_agent_slots,
      input_queue_items,
      messages,
      set_error,
      set_messages,
      set_pending_agent_slots,
      set_input_queue_items,
      set_pending_permissions,
    }),
    [
      identity,
      session_key,
      ws_state,
      session_control_state,
      ws_send,
      pending_permissions,
      pending_agent_slots,
      input_queue_items,
      messages,
      set_error,
      set_messages,
      set_pending_agent_slots,
      set_input_queue_items,
      set_pending_permissions,
    ],
  );

  const send_message = useCallback(
    async (content: string, options: AgentConversationSendOptions = {}) => {
      const round_id = await send_session_message(content, action_context, options);
      if (!round_id) {
        return;
      }

      apply_runtime_transition((machine) => {
        machine.track_outbound_round(round_id);
      });

      await new Promise<void>((resolve, reject) => {
        const timeout_id = window.setTimeout(() => {
          fail_pending_chat_ack(round_id, "消息未送达后端，请重试");
        }, get_message_send_ack_timeout_ms());
        pending_chat_ack_ref.current.set(round_id, {
          resolve,
          reject,
          timeout_id,
        });
      });
    },
    [action_context, apply_runtime_transition, fail_pending_chat_ack],
  );

  const enqueue_input_queue_message = useCallback(
    async (
      content: string,
      delivery_policy: AgentConversationDeliveryPolicy = "queue",
    ) => {
      send_enqueue_input_queue_message(content, action_context, delivery_policy);
    },
    [action_context],
  );

  const delete_input_queue_message = useCallback(
    async (item_id: string) => {
      send_delete_input_queue_message(item_id, action_context);
    },
    [action_context],
  );

  const guide_input_queue_message = useCallback(
    async (item_id: string) => {
      send_guide_input_queue_message(item_id, action_context);
    },
    [action_context],
  );

  const reorder_input_queue_messages = useCallback(
    async (ordered_ids: string[]) => {
      send_reorder_input_queue_messages(ordered_ids, action_context);
    },
    [action_context],
  );

  const stop_generation = useCallback(
    (msg_id?: string) => {
      stop_session_generation(action_context, msg_id);
      if (msg_id) {
        apply_runtime_transition((machine) => {
          machine.update_message_status(msg_id, "cancelled");
        });
        set_pending_agent_slots((prev) =>
          prev.map((slot) =>
            slot.msg_id === msg_id
              ? {
                  ...slot,
                  status: "cancelled",
                }
              : slot,
          ),
        );
        return;
      }
    },
    [action_context, apply_runtime_transition, set_pending_agent_slots],
  );

  const send_permission_response = useCallback(
    (payload: PermissionDecisionPayload) => {
      return send_session_permission_response(payload, action_context);
    },
    [action_context],
  );

  const start_session = useCallback(() => {
    cancel_pending_chat_acks("会话已重建，未确认的消息发送已取消");
    start_agent_session(lifecycle_context);
    reset_history_state();
    set_history_prepend_token(0);
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_state,
    reset_runtime_machine,
  ]);

  const load_session = useCallback(
    async (id: string): Promise<void> => {
      await load_agent_session(id, lifecycle_context);
    },
    [lifecycle_context],
  );

  const clear_session = useCallback(() => {
    cancel_pending_chat_acks("会话已清空，未确认的消息发送已取消");
    clear_agent_session(lifecycle_context);
    reset_history_state();
    set_history_prepend_token(0);
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_state,
    reset_runtime_machine,
  ]);

  const bind_session_key = useCallback(
    (key: string | null) => {
      const normalized_key = key?.trim() || null;
      if (active_session_key_ref.current === normalized_key) {
        return;
      }

      active_session_key_ref.current = normalized_key;
      cancel_pending_chat_acks("会话已切换，未确认的消息发送已取消");
      reset_history_state();
      set_history_prepend_token(0);
      set_session_key((current_key) =>
        current_key === normalized_key ? current_key : normalized_key,
      );
      if (!normalized_key) {
        set_is_session_loading(false);
        reset_runtime_machine();
        reset_session_control();
        set_pending_agent_slots((current_slots) =>
          current_slots.length ? [] : current_slots,
        );
        set_input_queue_items((current_items) =>
          current_items.length ? [] : current_items,
        );
        set_pending_permissions((current_permissions) =>
          current_permissions.length ? [] : current_permissions,
        );
      }
    },
    [
      cancel_pending_chat_acks,
      reset_history_state,
      reset_runtime_machine,
      reset_session_control,
      set_is_session_loading,
      set_pending_agent_slots,
      set_input_queue_items,
      set_pending_permissions,
    ],
  );

  const reset_session = useCallback(() => {
    cancel_pending_chat_acks("会话已重置，未确认的消息发送已取消");
    reset_agent_session(lifecycle_context);
    reset_history_state();
    set_history_prepend_token(0);
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_state,
    reset_runtime_machine,
  ]);

  return {
    error,
    messages,
    session_key,
    ws_state,
    is_loading,
    live_round_ids,
    is_session_loading,
    is_history_loading,
    has_more_history,
    history_prepend_token,
    runtime_phase,
    session_control_state,
    is_session_controller,
    session_controller_client_id,
    session_observer_count,
    pending_agent_slots,
    input_queue_items,
    pending_permissions,
    send_message,
    enqueue_input_queue_message,
    delete_input_queue_message,
    guide_input_queue_message,
    reorder_input_queue_messages,
    bind_session_key,
    start_session,
    load_session,
    load_older_messages,
    clear_session,
    reset_session,
    stop_generation,
    send_permission_response,
  };
}

export type {
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
