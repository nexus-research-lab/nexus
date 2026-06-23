import {
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from "react";
import {
  get_agent_ws_url,
} from "@/config/options";
import { are_equivalent_session_keys } from "@/lib/conversation/session-key";
import { useAgentStore } from "@/store/agent";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import {
  Message,
  RoundLifecycleStatus,
  SessionStatusEventPayload,
  WebSocketMessage,
  WebSocketState,
} from "@/types";
import {
  PermissionDecisionPayload,
} from "@/types/conversation/permission";
import {
  AgentConversationActionContext,
  AgentConversationDeliveryPolicy,
  AgentConversationLifecycleContext,
  AgentConversationSendOptions,
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
import {
  clear_agent_session,
  load_agent_session,
  reset_agent_session,
  start_agent_session,
} from "./conversation-lifecycle";
import {
  dedupe_messages_by_id,
  merge_loaded_messages,
  upsert_message,
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
import { are_runtime_snapshots_equal } from "./conversation-runtime-state";
import {
  apply_terminal_round_message_status,
  cancel_running_agent_slots,
  filter_round_pending_agent_slots,
  filter_round_pending_permissions,
  merge_chat_ack_pending_slots,
  reconcile_stopped_session_messages,
  remove_failed_outbound_user_message,
  update_assistant_message_status,
  update_pending_agent_slot_status,
} from "./conversation-runtime-reconciliation";
import {
  AgentConversationHistoryCursor,
  load_older_agent_conversation_messages,
} from "./conversation-history";
import {
  build_volatile_conversation_snapshot,
  filter_pending_permissions_from_snapshot,
  filter_pending_slots_from_snapshot,
  get_next_pending_permission_timeout_ms,
  is_ephemeral_message,
  merge_pending_agent_slots,
  prune_expired_pending_permissions,
  read_volatile_conversation_snapshot,
  remove_volatile_conversation_snapshot,
  write_volatile_conversation_snapshot,
} from "./conversation-volatile-snapshot";
import { useConversationStreamBuffer } from "./use-conversation-stream-buffer";
import { usePendingChatAcks } from "./use-pending-chat-acks";
import { useAgentConversationSocket } from "./use-agent-conversation-socket";

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
  const settle_agent_workspace_writes = useWorkspaceLiveStore(
    (state) => state.settle_agent_writes,
  );
  const agent_runtime_status = useAgentStore((state) => (
    agent_id ? state.agent_runtime_statuses[agent_id] : undefined
  ));
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

  const active_session_key_ref = useRef<string | null>(
    identity?.session_key ?? null,
  );
  const active_identity_key_ref = useRef<string | null>(
    get_agent_conversation_identity_key(identity),
  );
  const load_request_id_ref = useRef(0);
  const session_seq_cursor_ref = useRef(0);
  const room_seq_cursor_ref = useRef(0);
  const is_history_loading_ref = useRef(false);
  const has_more_history_ref = useRef(false);
  const history_cursor_ref = useRef<AgentConversationHistoryCursor>({
    before_round_id: null,
    before_round_timestamp: null,
  });
  const pending_agent_slots_ref = useRef<RoomPendingAgentSlotState[]>([]);
  const pending_permissions_ref = useRef<
    UseAgentConversationReturn["pending_permissions"]
  >([]);
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

  const reset_history_pagination = useCallback(() => {
    reset_history_state();
    set_history_prepend_token(0);
  }, [reset_history_state]);

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

  const clear_live_session_state = useCallback(() => {
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
    set_input_queue_items,
    set_pending_agent_slots,
    set_pending_permissions,
  ]);

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

  const is_current_room_event = useCallback(
    (incoming_room_id?: string | null) => {
      if (!incoming_room_id || !room_id) {
        return false;
      }
      return incoming_room_id === room_id;
    },
    [room_id],
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

  const {
    cancel_pending_chat_acks,
    clear_pending_chat_ack,
    reject_pending_chat_ack,
    wait_for_chat_ack,
  } = usePendingChatAcks();

  const fail_pending_chat_ack = useCallback(
    (round_id: string, message: string) => {
      if (!reject_pending_chat_ack(round_id, message)) {
        return;
      }
      apply_runtime_transition((machine) => {
        machine.clear_round(round_id, chat_type === "group");
      });
      set_pending_agent_slots((prev) =>
        filter_round_pending_agent_slots(prev, round_id),
      );
      set_pending_permissions((prev) =>
        filter_round_pending_permissions(prev, round_id),
      );
      set_messages((prev) =>
        remove_failed_outbound_user_message(prev, round_id),
      );
      set_error(message);
      if (ws_state_ref.current === "connected") {
        ws_reconnect_ref.current();
      }
    },
    [
      apply_runtime_transition,
      chat_type,
      reject_pending_chat_ack,
      set_messages,
      set_pending_agent_slots,
      set_pending_permissions,
    ],
  );

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
    return load_older_agent_conversation_messages({
      active_session_key_ref,
      identity,
      history_cursor_ref,
      has_more_history_ref,
      is_history_loading_ref,
      set_history_loading,
      set_has_more_history,
      set_history_prepend_token,
      set_messages,
      set_error,
    });
  }, [
    identity,
    set_error,
    set_has_more_history,
    set_history_loading,
    set_messages,
  ]);

  const enqueue_stream_payload = useConversationStreamBuffer(set_messages);

  const reconcile_stopped_session = useCallback(() => {
    const runtime_snapshot_before_reset =
      runtime_machine_ref.current.snapshot();
    apply_runtime_transition((machine) => {
      machine.reset();
    });
    if (agent_id) {
      settle_agent_workspace_writes(agent_id);
    }
    set_pending_permissions([]);
    set_pending_agent_slots(cancel_running_agent_slots);
    set_messages((prev) =>
      reconcile_stopped_session_messages(
        prev,
        runtime_snapshot_before_reset.terminal_round_ids,
        chat_type,
      ),
    );
  }, [
    apply_runtime_transition,
    agent_id,
    chat_type,
    settle_agent_workspace_writes,
    set_messages,
    set_pending_agent_slots,
    set_pending_permissions,
  ]);

  const sync_session_status = useCallback(
    (payload: SessionStatusEventPayload) => {
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
        update_assistant_message_status(prev, msg_id, status),
      );
      set_pending_agent_slots((prev) =>
        update_pending_agent_slot_status(prev, msg_id, status, round_id),
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
      set_pending_agent_slots((prev) => merge_chat_ack_pending_slots(prev, ack));
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
      if (agent_id && !runtime_machine_ref.current.snapshot().is_loading) {
        settle_agent_workspace_writes(agent_id);
      }

      set_pending_permissions((prev) =>
        filter_round_pending_permissions(prev, round_id),
      );
      set_pending_agent_slots((prev) =>
        filter_round_pending_agent_slots(prev, round_id),
      );
      set_messages((prev) =>
        apply_terminal_round_message_status(prev, round_id, status),
      );
    },
    [
      apply_runtime_transition,
      agent_id,
      clear_pending_chat_ack,
      settle_agent_workspace_writes,
      set_messages,
      set_pending_agent_slots,
      set_pending_permissions,
    ],
  );

  const handle_websocket_message = useCallback(
    (backend_message: unknown) => {
      handle_agent_conversation_web_socket_message({
        backend_message,
        agent_id,
        room_id,
        conversation_id,
        session_key,
        session_seq_cursor_ref,
        room_seq_cursor_ref,
        ws_state_ref,
        ws_send_ref,
        apply_workspace_event,
        is_current_room_event,
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
        reload_current_session,
        settle_agent_workspace_writes,
      });
    },
    [
      apply_workspace_event,
      is_current_room_event,
      is_current_session_event,
      enqueue_stream_payload,
      on_background_message,
      on_room_event,
      room_id,
      agent_id,
      session_key,
      conversation_id,
      reload_current_session,
      apply_round_status,
      settle_agent_workspace_writes,
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
    reset_history_pagination();
    clear_live_session_state();
  }, [
    cancel_pending_chat_acks,
    clear_live_session_state,
    identity,
    reset_history_pagination,
    reset_runtime_machine,
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

  useEffect(() => {
    return () => {
      cancel_pending_chat_acks("会话已卸载，未确认的消息发送已取消");
    };
  }, [cancel_pending_chat_acks]);

  const { ws_state, ws_send } = useAgentConversationSocket({
    ws_url,
    agent_id,
    room_id,
    conversation_id,
    session_key,
    session_seq_cursor_ref,
    room_seq_cursor_ref,
    ws_send_ref,
    ws_reconnect_ref,
    ws_state_ref,
    on_message: handle_websocket_message,
    on_error,
    set_error,
  });

  useEffect(() => {
    if (
      agent_id &&
      agent_runtime_status?.running_task_count === 0 &&
      agent_runtime_status.status !== "running"
    ) {
      settle_agent_workspace_writes(agent_id);
    }
  }, [agent_id, agent_runtime_status, settle_agent_workspace_writes]);

  const action_context: AgentConversationActionContext = useMemo(
    () => ({
      identity,
      session_key,
      ws_state,
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

      await wait_for_chat_ack(round_id, () => {
        fail_pending_chat_ack(round_id, "消息未送达后端，请重试");
      });
    },
    [
      action_context,
      apply_runtime_transition,
      fail_pending_chat_ack,
      wait_for_chat_ack,
    ],
  );

  const enqueue_input_queue_message = useCallback(
    async (
      content: string,
      delivery_policy: AgentConversationDeliveryPolicy = "queue",
      attachments: AgentConversationSendOptions["attachments"] = [],
    ) => {
      send_enqueue_input_queue_message(content, action_context, delivery_policy, attachments);
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
    reset_history_pagination();
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_pagination,
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
    reset_history_pagination();
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_pagination,
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
      reset_history_pagination();
      set_session_key((current_key) =>
        current_key === normalized_key ? current_key : normalized_key,
      );
      if (!normalized_key) {
        set_is_session_loading(false);
        reset_runtime_machine();
        clear_live_session_state();
      }
    },
    [
      cancel_pending_chat_acks,
      clear_live_session_state,
      reset_history_pagination,
      reset_runtime_machine,
      set_is_session_loading,
    ],
  );

  const reset_session = useCallback(() => {
    cancel_pending_chat_acks("会话已重置，未确认的消息发送已取消");
    reset_agent_session(lifecycle_context);
    reset_history_pagination();
    reset_runtime_machine();
  }, [
    cancel_pending_chat_acks,
    lifecycle_context,
    reset_history_pagination,
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
