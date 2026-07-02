import {
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";
import {
  get_agent_ws_url,
} from "@/config/options";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
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
} from "./agent-conversation-runtime-machine";
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
  const wsUrl = options.ws_url || get_agent_ws_url();
  const identity = options.identity ?? null;
  const agentId = identity?.agent_id ?? null;
  const roomId = identity?.room_id ?? null;
  const conversationId = identity?.conversation_id ?? null;
  const chatType = identity?.chat_type ?? "dm";
  const onError = options.on_error;
  const onRoomEventCallback = options.on_room_event;
  const applyWorkspaceEvent = useWorkspaceLiveStore(
    (state) => state.apply_event,
  );
  const settleAgentWorkspaceWrites = useWorkspaceLiveStore(
    (state) => state.settle_agent_writes,
  );
  const agentRuntimeStatus = useAgentStore((state) => (
    agentId ? state.agent_runtime_statuses[agentId] : undefined
  ));
  const runtimeMachineRef = useRef(
    new AgentConversationRuntimeMachine(chatType),
  );
  const runtimeSnapshot = useSyncExternalStore(
    useCallback((cb) => runtimeMachineRef.current.subscribe(cb), []),
    useCallback(() => runtimeMachineRef.current.snapshot(), []),
  );
  const identitySessionKey = identity?.session_key?.trim() || null;

  const [messages, setMessagesState] = useState<Message[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [sessionKey, setSessionKey] = useResettableState<string | null>(identitySessionKey, identitySessionKey);
  const [isSessionLoading, setIsSessionLoading] = useState(false);
  const [isHistoryLoading, setIsHistoryLoadingState] = useState(false);
  const [hasMoreHistory, setHasMoreHistoryState] = useState(false);
  const [historyPrependToken, setHistoryPrependToken] = useState(0);
  const [pendingAgentSlots, setPendingAgentSlotsState] = useState<
    RoomPendingAgentSlotState[]
  >([]);
  const [inputQueueItems, setInputQueueItemsState] = useState<
    InputQueueItem[]
  >([]);
  const [pendingPermissions, setPendingPermissionsState] = useState<
    UseAgentConversationReturn["pending_permissions"]
  >([]);

  const activeSessionKeyRef = useRef<string | null>(identitySessionKey);
  const activeIdentityKeyRef = useRef<string | null>(
    get_agent_conversation_identity_key(identity),
  );
  const loadRequestIdRef = useRef(0);
  const sessionSeqCursorRef = useRef(0);
  const roomSeqCursorRef = useRef(0);
  const isHistoryLoadingRef = useRef(false);
  const hasMoreHistoryRef = useRef(false);
  const historyCursorRef = useRef<AgentConversationHistoryCursor>({
    before_round_id: null,
    before_round_timestamp: null,
  });
  const pendingAgentSlotsRef = useRef<RoomPendingAgentSlotState[]>([]);
  const pendingPermissionsRef = useRef<
    UseAgentConversationReturn["pending_permissions"]
  >([]);
  const wsSendRef = useRef<
    (payload: WebSocketMessage) => {
      disposition: "sent" | "queued" | "dropped";
    }
  >(() => ({ disposition: "dropped" }));
  const wsReconnectRef = useRef<() => void>(() => {});
  const wsStateRef = useRef<WebSocketState>("disconnected");
  // Per-session message cache: accumulates messages received for non-active sessions
  // so they are not lost when the user switches conversations.
  const bgMessageCacheRef = useRef<Map<string, Message[]>>(new Map());
  const isLoading = runtimeSnapshot.is_loading;
  const runtimePhase = runtimeSnapshot.phase;
  const liveRoundIds = runtimeSnapshot.live_round_ids;

  const setMessages = useCallback((nextState: SetStateAction<Message[]>) => {
    setMessagesState((currentMessages) => {
      const nextMessages =
        typeof nextState === "function"
          ? nextState(currentMessages)
          : nextState;
      return dedupe_messages_by_id(nextMessages);
    });
  }, []);

  const setHistoryLoading = useCallback((nextValue: boolean) => {
    isHistoryLoadingRef.current = nextValue;
    setIsHistoryLoadingState((currentValue) =>
      currentValue === nextValue ? currentValue : nextValue,
    );
  }, []);

  const setHasMoreHistory = useCallback((nextValue: boolean) => {
    hasMoreHistoryRef.current = nextValue;
    setHasMoreHistoryState((currentValue) =>
      currentValue === nextValue ? currentValue : nextValue,
    );
  }, []);

  const resetHistoryState = useCallback(() => {
    historyCursorRef.current = {
      before_round_id: null,
      before_round_timestamp: null,
    };
    setHistoryLoading(false);
    setHasMoreHistory(false);
  }, [setHasMoreHistory, setHistoryLoading]);

  const resetHistoryPagination = useCallback(() => {
    resetHistoryState();
    setHistoryPrependToken(0);
  }, [resetHistoryState]);

  const applyRuntimeTransition = useCallback(
    (transition: (machine: AgentConversationRuntimeMachine) => void) => {
      transition(runtimeMachineRef.current);
      runtimeMachineRef.current.emit();
    },
    [],
  );

  const setPendingAgentSlots = useCallback(
    (nextState: SetStateAction<RoomPendingAgentSlotState[]>) => {
      const next =
        typeof nextState === "function"
          ? nextState(pendingAgentSlotsRef.current)
          : nextState;
      pendingAgentSlotsRef.current = next;
      setPendingAgentSlotsState(next);
    },
    [],
  );

  const setInputQueueItems = useCallback(
    (nextState: SetStateAction<InputQueueItem[]>) => {
      setInputQueueItemsState((currentItems) =>
        typeof nextState === "function"
          ? nextState(currentItems)
          : nextState,
      );
    },
    [],
  );

  const setPendingPermissions = useCallback(
    (
      nextState: SetStateAction<
        UseAgentConversationReturn["pending_permissions"]
      >,
    ) => {
      const next =
        typeof nextState === "function"
          ? nextState(pendingPermissionsRef.current)
          : nextState;
      pendingPermissionsRef.current = next;
      applyRuntimeTransition((machine) => {
        machine.set_pending_permission_count(next.length);
      });
      setPendingPermissionsState(next);
    },
    [applyRuntimeTransition],
  );

  const clearLiveSessionState = useCallback(() => {
    setPendingAgentSlots((currentSlots) =>
      currentSlots.length ? [] : currentSlots,
    );
    setInputQueueItems((currentItems) =>
      currentItems.length ? [] : currentItems,
    );
    setPendingPermissions((currentPermissions) =>
      currentPermissions.length ? [] : currentPermissions,
    );
  }, [
    setInputQueueItems,
    setPendingAgentSlots,
    setPendingPermissions,
  ]);

  const isCurrentSessionEvent = useCallback(
    (incomingSessionKey?: string | null) => {
      if (!incomingSessionKey) {
        return false;
      }
      return are_equivalent_session_keys(
        activeSessionKeyRef.current,
        incomingSessionKey,
      );
    },
    [],
  );

  const isCurrentRoomEvent = useCallback(
    (incomingRoomId?: string | null) => {
      if (!incomingRoomId || !roomId) {
        return false;
      }
      return incomingRoomId === roomId;
    },
    [roomId],
  );

  const onBackgroundMessage = useCallback((key: string, message: Message) => {
    if (is_ephemeral_message(message)) {
      return;
    }
    const cache = bgMessageCacheRef.current;
    const existing = cache.get(key) ?? [];
    const next = upsert_message(existing, message);
    cache.set(key, next);
  }, []);

  const onRoomEvent = useCallback(
    (eventType: string, data: RoomEventPayload) => {
      onRoomEventCallback?.(eventType, data);
    },
    [onRoomEventCallback],
  );

  const {
    cancel_pending_chat_acks: cancelPendingChatAcks,
    clear_pending_chat_ack: clearPendingChatAck,
    reject_pending_chat_ack: rejectPendingChatAck,
    wait_for_chat_ack: waitForChatAck,
  } = usePendingChatAcks();

  const failPendingChatAck = useCallback(
    (roundId: string, message: string) => {
      if (!rejectPendingChatAck(roundId, message)) {
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.clear_round(roundId, chatType === "group");
      });
      setPendingAgentSlots((prev) =>
        filter_round_pending_agent_slots(prev, roundId),
      );
      setPendingPermissions((prev) =>
        filter_round_pending_permissions(prev, roundId),
      );
      setMessages((prev) =>
        remove_failed_outbound_user_message(prev, roundId),
      );
      setError(message);
      if (wsStateRef.current === "connected") {
        wsReconnectRef.current();
      }
    },
    [
      applyRuntimeTransition,
      chatType,
      rejectPendingChatAck,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
    ],
  );

  const resetRuntimeMachine = useCallback(() => {
    applyRuntimeTransition((machine) => {
      machine.reset();
    });
  }, [applyRuntimeTransition]);

  const reconcileRuntimeStateFromSnapshot = useCallback(
    (snapshotMessages: Message[]) => {
      applyRuntimeTransition((machine) => {
        machine.reconcile_from_snapshot(snapshotMessages);
      });
      const isRoundTerminal = (roundId: string) =>
        runtimeMachineRef.current.is_round_terminal(roundId);

      setPendingAgentSlots(
        filter_pending_slots_from_snapshot(
          pendingAgentSlotsRef.current,
          snapshotMessages,
          isRoundTerminal,
        ),
      );
      setPendingPermissions(
        filter_pending_permissions_from_snapshot(
          pendingPermissionsRef.current,
          snapshotMessages,
          isRoundTerminal,
        ),
      );
    },
    [
      applyRuntimeTransition,
      setPendingAgentSlots,
      setPendingPermissions,
    ],
  );

  const lifecycleContext: AgentConversationLifecycleContext = useMemo(
    () => ({
      active_session_key_ref: activeSessionKeyRef,
      load_request_id_ref: loadRequestIdRef,
      identity,
      set_session_key: setSessionKey,
      set_is_session_loading: setIsSessionLoading,
      set_messages: setMessages,
      set_pending_agent_slots: setPendingAgentSlots,
      set_input_queue_items: setInputQueueItems,
      set_pending_permissions: setPendingPermissions,
      set_error: setError,
      bg_message_cache_ref: bgMessageCacheRef,
      restore_volatile_session_snapshot: (targetSessionKey) => {
        const snapshot =
          read_volatile_conversation_snapshot(targetSessionKey);
        if (!snapshot) {
          return false;
        }

        let restoredMessages = snapshot.messages;
        setMessages((currentMessages) => {
          restoredMessages = merge_loaded_messages(
            snapshot.messages,
            currentMessages,
          );
          return restoredMessages;
        });
        setPendingAgentSlots((currentSlots) =>
          merge_pending_agent_slots(
            snapshot.pending_agent_slots,
            currentSlots,
          ),
        );
        setError(null);
        reconcileRuntimeStateFromSnapshot(restoredMessages);
        return (
          restoredMessages.length > 0 ||
          snapshot.pending_agent_slots.length > 0
        );
      },
      on_session_messages_loaded: (loadedMessages, meta) => {
        if (!meta.is_reload) {
          historyCursorRef.current = {
            before_round_id: meta.next_before_round_id,
            before_round_timestamp: meta.next_before_round_timestamp,
          };
          setHasMoreHistory(meta.has_more_history);
        }
        reconcileRuntimeStateFromSnapshot(loadedMessages);
      },
    }),
    [
      activeSessionKeyRef,
      loadRequestIdRef,
      identity,
      setSessionKey,
      setIsSessionLoading,
      setMessages,
      setPendingAgentSlots,
      setInputQueueItems,
      setPendingPermissions,
      setError,
      bgMessageCacheRef,
      reconcileRuntimeStateFromSnapshot,
      setHasMoreHistory,
    ],
  );

  useEffect(() => {
    if (!sessionKey) {
      return;
    }

    const snapshot = build_volatile_conversation_snapshot(
      messages,
      runtimeSnapshot,
      pendingAgentSlots,
    );
    if (!snapshot) {
      remove_volatile_conversation_snapshot(sessionKey);
      return;
    }

    write_volatile_conversation_snapshot(sessionKey, snapshot);
  }, [messages, pendingAgentSlots, runtimeSnapshot, sessionKey]);

  useEffect(() => {
    const nextPermissions = prune_expired_pending_permissions(
      pendingPermissionsRef.current,
    );
    if (nextPermissions !== pendingPermissionsRef.current) {
      setPendingPermissions(nextPermissions);
      return;
    }

    const nextTimeoutMs = get_next_pending_permission_timeout_ms(
      pendingPermissionsRef.current,
    );
    if (nextTimeoutMs == null) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setPendingPermissions((currentPermissions) =>
        prune_expired_pending_permissions(currentPermissions),
      );
    }, nextTimeoutMs + 1);

    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [pendingPermissions, setPendingPermissions]);

  const reloadCurrentSession = useCallback(async () => {
    const activeSessionKey = activeSessionKeyRef.current;
    if (!activeSessionKey) {
      return;
    }

    await load_agent_session(activeSessionKey, lifecycleContext, true);
  }, [lifecycleContext]);

  const loadOlderMessages = useCallback(async (): Promise<boolean> => {
    return load_older_agent_conversation_messages({
      active_session_key_ref: activeSessionKeyRef,
      identity,
      history_cursor_ref: historyCursorRef,
      has_more_history_ref: hasMoreHistoryRef,
      is_history_loading_ref: isHistoryLoadingRef,
      set_history_loading: setHistoryLoading,
      set_has_more_history: setHasMoreHistory,
      set_history_prepend_token: setHistoryPrependToken,
      set_messages: setMessages,
      set_error: setError,
    });
  }, [
    identity,
    setError,
    setHasMoreHistory,
    setHistoryLoading,
    setMessages,
  ]);

  const enqueueStreamPayload = useConversationStreamBuffer(setMessages);

  const reconcileStoppedSession = useCallback(() => {
    const runtimeSnapshotBeforeReset =
      runtimeMachineRef.current.snapshot();
    applyRuntimeTransition((machine) => {
      machine.reset();
    });
    if (agentId) {
      settleAgentWorkspaceWrites(agentId);
    }
    setPendingPermissions([]);
    setPendingAgentSlots(cancel_running_agent_slots);
    setMessages((prev) =>
      reconcile_stopped_session_messages(
        prev,
        runtimeSnapshotBeforeReset.terminal_round_ids,
        chatType,
      ),
    );
  }, [
    applyRuntimeTransition,
    agentId,
    chatType,
    settleAgentWorkspaceWrites,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
  ]);

  const syncSessionStatus = useCallback(
    (payload: SessionStatusEventPayload) => {
      const runningRoundIds = Array.isArray(payload.running_round_ids)
        ? payload.running_round_ids.filter(
            (roundId): roundId is string => typeof roundId === "string",
          )
        : [];
      if (!payload.is_generating || runningRoundIds.length === 0) {
        reconcileStoppedSession();
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.sync_running_rounds(runningRoundIds);
      });
    },
    [applyRuntimeTransition, reconcileStoppedSession],
  );

  const updateMessageStatus = useCallback(
    (
      msgId: string,
      status: AssistantMessageStatus,
      roundId?: string | null,
    ) => {
      setMessages((prev) =>
        update_assistant_message_status(prev, msgId, status),
      );
      setPendingAgentSlots((prev) =>
        update_pending_agent_slot_status(prev, msgId, status, roundId),
      );
      applyRuntimeTransition((machine) => {
        machine.update_message_status(msgId, status, roundId);
      });
    },
    [applyRuntimeTransition, setMessages, setPendingAgentSlots],
  );

  const trackChatAck = useCallback(
    (ack: import("@/types").ChatAckData, _sessionKey?: string | null) => {
      applyRuntimeTransition((machine) => {
        machine.track_chat_ack(ack);
      });
      clearPendingChatAck(ack.round_id);
      setPendingAgentSlots((prev) => merge_chat_ack_pending_slots(prev, ack));
    },
    [applyRuntimeTransition, clearPendingChatAck, setPendingAgentSlots],
  );

  const trackAssistantMessage = useCallback(
    (message: AssistantMessage) => {
      clearPendingChatAck(message.round_id);
      applyRuntimeTransition((machine) => {
        machine.track_assistant_message(message);
      });
    },
    [applyRuntimeTransition, clearPendingChatAck],
  );

  const applyRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus) => {
      applyRuntimeTransition((machine) => {
        machine.track_round_status(roundId, status);
      });
      clearPendingChatAck(roundId);

      if (status === "running") {
        return;
      }
      if (agentId && !runtimeMachineRef.current.snapshot().is_loading) {
        settleAgentWorkspaceWrites(agentId);
      }

      setPendingPermissions((prev) =>
        filter_round_pending_permissions(prev, roundId),
      );
      setPendingAgentSlots((prev) =>
        filter_round_pending_agent_slots(prev, roundId),
      );
      setMessages((prev) =>
        apply_terminal_round_message_status(prev, roundId, status),
      );
    },
    [
      applyRuntimeTransition,
      agentId,
      clearPendingChatAck,
      settleAgentWorkspaceWrites,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
    ],
  );

  const handleWebsocketMessage = useCallback(
    (backendMessage: unknown) => {
      handle_agent_conversation_web_socket_message({
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
        set_pending_agent_slots: setPendingAgentSlots,
        set_input_queue_items: setInputQueueItems,
        set_pending_permissions: setPendingPermissions,
        enqueue_stream_payload: enqueueStreamPayload,
        on_background_message: onBackgroundMessage,
        on_room_event: onRoomEvent,
        update_message_status: updateMessageStatus,
        sync_session_status: syncSessionStatus,
        apply_round_status: applyRoundStatus,
        track_chat_ack: trackChatAck,
        track_assistant_message: trackAssistantMessage,
        reload_current_session: reloadCurrentSession,
        settle_agent_workspace_writes: settleAgentWorkspaceWrites,
      });
    },
    [
      applyWorkspaceEvent,
      isCurrentRoomEvent,
      isCurrentSessionEvent,
      enqueueStreamPayload,
      onBackgroundMessage,
      onRoomEvent,
      roomId,
      agentId,
      sessionKey,
      conversationId,
      reloadCurrentSession,
      applyRoundStatus,
      settleAgentWorkspaceWrites,
      setPendingAgentSlots,
      setInputQueueItems,
      setMessages,
      setPendingPermissions,
      syncSessionStatus,
      trackAssistantMessage,
      trackChatAck,
      updateMessageStatus,
    ],
  );

  useEffect(() => {
    runtimeMachineRef.current.set_chat_type(chatType);
    runtimeMachineRef.current.emit();
  }, [chatType]);

  const nextIdentityKey = get_agent_conversation_identity_key(identity);
  const shouldResetIdentityState = activeIdentityKeyRef.current !== nextIdentityKey;
  if (shouldResetIdentityState) {
    activeIdentityKeyRef.current = nextIdentityKey;
    sessionSeqCursorRef.current = 0;
    roomSeqCursorRef.current = 0;
    resetHistoryPagination();
    clearLiveSessionState();
  }

  useEffect(() => {
    if (!shouldResetIdentityState) {
      return;
    }
    cancelPendingChatAcks("会话上下文已切换，未确认的消息发送已取消");
    resetRuntimeMachine();
  }, [
    cancelPendingChatAcks,
    shouldResetIdentityState,
    resetRuntimeMachine,
  ]);

  useEffect(() => {
    activeSessionKeyRef.current = identitySessionKey;
  }, [identitySessionKey]);

  useEffect(() => {
    return () => {
      cancelPendingChatAcks("会话已卸载，未确认的消息发送已取消");
    };
  }, [cancelPendingChatAcks]);

  const { ws_state: wsState, ws_send: wsSend } = useAgentConversationSocket({
    ws_url: wsUrl,
    agent_id: agentId,
    room_id: roomId,
    conversation_id: conversationId,
    session_key: sessionKey,
    session_seq_cursor_ref: sessionSeqCursorRef,
    room_seq_cursor_ref: roomSeqCursorRef,
    ws_send_ref: wsSendRef,
    ws_reconnect_ref: wsReconnectRef,
    ws_state_ref: wsStateRef,
    on_message: handleWebsocketMessage,
    on_error: onError,
    set_error: setError,
  });

  useEffect(() => {
    if (
      agentId &&
      agentRuntimeStatus?.running_task_count === 0 &&
      agentRuntimeStatus.status !== "running"
    ) {
      settleAgentWorkspaceWrites(agentId);
    }
  }, [agentId, agentRuntimeStatus, settleAgentWorkspaceWrites]);

  const actionContext: AgentConversationActionContext = useMemo(
    () => ({
      identity,
      session_key: sessionKey,
      ws_state: wsState,
      ws_send: wsSend,
      active_session_key_ref: activeSessionKeyRef,
      pending_permissions: pendingPermissions,
      pending_agent_slots: pendingAgentSlots,
      input_queue_items: inputQueueItems,
      messages,
      set_error: setError,
      set_messages: setMessages,
      set_pending_agent_slots: setPendingAgentSlots,
      set_input_queue_items: setInputQueueItems,
      set_pending_permissions: setPendingPermissions,
    }),
    [
      identity,
      sessionKey,
      wsState,
      wsSend,
      pendingPermissions,
      pendingAgentSlots,
      inputQueueItems,
      messages,
      setError,
      setMessages,
      setPendingAgentSlots,
      setInputQueueItems,
      setPendingPermissions,
    ],
  );

  const sendMessage = useCallback(
    async (content: string, options: AgentConversationSendOptions = {}) => {
      const roundId = await send_session_message(content, actionContext, options);
      if (!roundId) {
        return;
      }

      applyRuntimeTransition((machine) => {
        machine.track_outbound_round(roundId);
      });

      await waitForChatAck(roundId, () => {
        failPendingChatAck(roundId, "消息未送达后端，请重试");
      });
    },
    [
      actionContext,
      applyRuntimeTransition,
      failPendingChatAck,
      waitForChatAck,
    ],
  );

  const enqueueInputQueueMessage = useCallback(
    async (
      content: string,
      deliveryPolicy: AgentConversationDeliveryPolicy = "queue",
      attachments: AgentConversationSendOptions["attachments"] = [],
    ) => {
      send_enqueue_input_queue_message(content, actionContext, deliveryPolicy, attachments);
    },
    [actionContext],
  );

  const deleteInputQueueMessage = useCallback(
    async (itemId: string) => {
      send_delete_input_queue_message(itemId, actionContext);
    },
    [actionContext],
  );

  const guideInputQueueMessage = useCallback(
    async (itemId: string) => {
      send_guide_input_queue_message(itemId, actionContext);
    },
    [actionContext],
  );

  const reorderInputQueueMessages = useCallback(
    async (orderedIds: string[]) => {
      send_reorder_input_queue_messages(orderedIds, actionContext);
    },
    [actionContext],
  );

  const stopGeneration = useCallback(
    (msgId?: string) => {
      stop_session_generation(actionContext, msgId);
      if (msgId) {
        applyRuntimeTransition((machine) => {
          machine.update_message_status(msgId, "cancelled");
        });
        setPendingAgentSlots((prev) =>
          prev.map((slot) =>
            slot.msg_id === msgId
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
    [actionContext, applyRuntimeTransition, setPendingAgentSlots],
  );

  const sendPermissionResponse = useCallback(
    (payload: PermissionDecisionPayload) => {
      return send_session_permission_response(payload, actionContext);
    },
    [actionContext],
  );

  const startSession = useCallback(() => {
    cancelPendingChatAcks("会话已重建，未确认的消息发送已取消");
    start_agent_session(lifecycleContext);
    resetHistoryPagination();
    resetRuntimeMachine();
  }, [
    cancelPendingChatAcks,
    lifecycleContext,
    resetHistoryPagination,
    resetRuntimeMachine,
  ]);

  const loadSession = useCallback(
    async (id: string): Promise<void> => {
      await load_agent_session(id, lifecycleContext);
    },
    [lifecycleContext],
  );

  const clearSession = useCallback(() => {
    cancelPendingChatAcks("会话已清空，未确认的消息发送已取消");
    clear_agent_session(lifecycleContext);
    resetHistoryPagination();
    resetRuntimeMachine();
  }, [
    cancelPendingChatAcks,
    lifecycleContext,
    resetHistoryPagination,
    resetRuntimeMachine,
  ]);

  const bindSessionKey = useCallback(
    (key: string | null) => {
      const normalizedKey = key?.trim() || null;
      if (activeSessionKeyRef.current === normalizedKey) {
        return;
      }

      activeSessionKeyRef.current = normalizedKey;
      cancelPendingChatAcks("会话已切换，未确认的消息发送已取消");
      resetHistoryPagination();
      setSessionKey((currentKey) =>
        currentKey === normalizedKey ? currentKey : normalizedKey,
      );
      if (!normalizedKey) {
        setIsSessionLoading(false);
        resetRuntimeMachine();
        clearLiveSessionState();
      }
    },
    [
      cancelPendingChatAcks,
      clearLiveSessionState,
      resetHistoryPagination,
      resetRuntimeMachine,
      setIsSessionLoading,
    ],
  );

  const resetSession = useCallback(() => {
    cancelPendingChatAcks("会话已重置，未确认的消息发送已取消");
    reset_agent_session(lifecycleContext);
    resetHistoryPagination();
    resetRuntimeMachine();
  }, [
    cancelPendingChatAcks,
    lifecycleContext,
    resetHistoryPagination,
    resetRuntimeMachine,
  ]);

  return {
    error,
    messages,
    session_key: sessionKey,
    ws_state: wsState,
    is_loading: isLoading,
    live_round_ids: liveRoundIds,
    is_session_loading: isSessionLoading,
    is_history_loading: isHistoryLoading,
    has_more_history: hasMoreHistory,
    history_prepend_token: historyPrependToken,
    runtime_phase: runtimePhase,
    pending_agent_slots: pendingAgentSlots,
    input_queue_items: inputQueueItems,
    pending_permissions: pendingPermissions,
    send_message: sendMessage,
    enqueue_input_queue_message: enqueueInputQueueMessage,
    delete_input_queue_message: deleteInputQueueMessage,
    guide_input_queue_message: guideInputQueueMessage,
    reorder_input_queue_messages: reorderInputQueueMessages,
    bind_session_key: bindSessionKey,
    start_session: startSession,
    load_session: loadSession,
    load_older_messages: loadOlderMessages,
    clear_session: clearSession,
    reset_session: resetSession,
    stop_generation: stopGeneration,
    send_permission_response: sendPermissionResponse,
  };
}

export type {
  UseAgentConversationOptions,
  UseAgentConversationReturn,
} from "@/types/agent/agent-conversation";
