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
  getAgentWsUrl,
} from "@/config/options";
import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
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
  getAgentConversationIdentityKey,
} from "@/types/agent/agent-conversation";
import {
  AssistantMessage,
  AssistantMessageStatus,
  RoomPendingAgentSlotState,
} from "@/types";
import {
  clearAgentSession,
  loadAgentSession,
  resetAgentSession,
  startAgentSession,
} from "./conversation-lifecycle";
import {
  dedupeMessagesById,
  mergeLoadedMessages,
  upsertMessage,
} from "./message-helpers";
import { handleAgentConversationWebSocketMessage } from "./websocket-event-handler";
import {
  deleteInputQueueMessage as send_delete_input_queue_message,
  enqueueInputQueueMessage as send_enqueue_input_queue_message,
  guideInputQueueMessage as send_guide_input_queue_message,
  reorderInputQueueMessages as send_reorder_input_queue_messages,
  rewriteLastUserMessage as send_rewrite_last_user_message,
  sendSessionMessage,
  sendSessionPermissionResponse,
  stopSessionGeneration,
} from "./conversation-actions";
import {
  AgentConversationRuntimeMachine,
} from "./agent-conversation-runtime-machine";
import {
  applyTerminalRoundMessageStatus,
  cancelRunningAgentSlots,
  filterAgentRoundPendingAgentSlots,
  filterRoundPendingAgentSlots,
  filterRoundPendingPermissions,
  mergeChatAckPendingSlots,
  reconcileStoppedSessionMessages,
  removeFailedOutboundUserMessage,
  removeRoundMessages,
  replaceOptimisticUserMessage,
  updateAssistantMessageStatus,
  updatePendingAgentSlotStatus,
} from "./conversation-runtime-reconciliation";
import {
  AgentConversationHistoryCursor,
  loadAgentConversationMessagesAroundRound,
  loadOlderAgentConversationMessages,
} from "./conversation-history";
import {
  buildVolatileConversationSnapshot,
  filterPendingPermissionsFromSnapshot,
  filterPendingSlotsFromSnapshot,
  getNextPendingPermissionTimeoutMs,
  isEphemeralMessage,
  mergePendingAgentSlots,
  pruneExpiredPendingPermissions,
  readVolatileConversationSnapshot,
  removeVolatileConversationSnapshot,
  writeVolatileConversationSnapshot,
} from "./conversation-volatile-snapshot";
import { useConversationStreamBuffer } from "./use-conversation-stream-buffer";
import { usePendingChatAcks } from "./use-pending-chat-acks";
import { useAgentConversationSocket } from "./use-agent-conversation-socket";

export function useAgentConversation(
  options: UseAgentConversationOptions = {},
): UseAgentConversationReturn {
  const wsUrl = options.ws_url || getAgentWsUrl();
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
    getAgentConversationIdentityKey(identity),
  );
  const loadRequestIdRef = useRef(0);
  const sessionSeqCursorRef = useRef(0);
  const roomSeqCursorRef = useRef(0);
  const isHistoryLoadingRef = useRef(false);
  const isRoundWindowLoadingRef = useRef(false);
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
  const isLoading = runtimeSnapshot.isLoading;
  const runtimePhase = runtimeSnapshot.phase;
  const liveRoundIds = runtimeSnapshot.liveRoundIds;

  const setMessages = useCallback((nextState: SetStateAction<Message[]>) => {
    setMessagesState((currentMessages) => {
      const nextMessages =
        typeof nextState === "function"
          ? nextState(currentMessages)
          : nextState;
      return dedupeMessagesById(nextMessages);
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
        machine.setPendingPermissionCount(next.length);
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
      return areEquivalentSessionKeys(
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
    if (isEphemeralMessage(message)) {
      return;
    }
    const cache = bgMessageCacheRef.current;
    const existing = cache.get(key) ?? [];
    const next = upsertMessage(existing, message);
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

  // ack 超时/失败：只按 client_request_id 拒绝、按 client_message_id 清理 optimistic 消息。
  // 此时可能还没有 canonical round_id，不做按 round 清理。
  const failPendingChatAck = useCallback(
    (clientRequestId: string, clientMessageId: string, message: string) => {
      if (!rejectPendingChatAck(clientRequestId, message)) {
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.clearOutboundRequest(clientRequestId);
      });
      setMessages((prev) =>
        removeFailedOutboundUserMessage(prev, clientMessageId),
      );
      setError(message);
      if (wsStateRef.current === "connected") {
        wsReconnectRef.current();
      }
    },
    [
      applyRuntimeTransition,
      rejectPendingChatAck,
      setMessages,
    ],
  );

  const settleChatAckWaitFailure = useCallback(
    (clientRequestId: string, clientMessageId: string, error: unknown) => {
      const message =
        error instanceof Error ? error.message : "消息未送达后端，请重试";
      applyRuntimeTransition((machine) => {
        machine.clearOutboundRequest(clientRequestId);
      });
      setMessages((prev) =>
        removeFailedOutboundUserMessage(prev, clientMessageId),
      );
      setError(message);
    },
    [
      applyRuntimeTransition,
      setError,
      setMessages,
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
        machine.reconcileFromSnapshot(snapshotMessages);
      });
      const isRoundTerminal = (roundId: string) =>
        runtimeMachineRef.current.isRoundTerminal(roundId);

      setPendingAgentSlots(
        filterPendingSlotsFromSnapshot(
          pendingAgentSlotsRef.current,
          snapshotMessages,
          isRoundTerminal,
        ),
      );
      setPendingPermissions(
        filterPendingPermissionsFromSnapshot(
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
          readVolatileConversationSnapshot(targetSessionKey);
        if (!snapshot) {
          return false;
        }

        let restoredMessages = snapshot.messages;
        setMessages((currentMessages) => {
          restoredMessages = mergeLoadedMessages(
            snapshot.messages,
            currentMessages,
          );
          return restoredMessages;
        });
        setPendingAgentSlots((currentSlots) =>
          mergePendingAgentSlots(
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

    const snapshot = buildVolatileConversationSnapshot(
      messages,
      runtimeSnapshot,
      pendingAgentSlots,
    );
    if (!snapshot) {
      removeVolatileConversationSnapshot(sessionKey);
      return;
    }

    writeVolatileConversationSnapshot(sessionKey, snapshot);
  }, [messages, pendingAgentSlots, runtimeSnapshot, sessionKey]);

  useEffect(() => {
    const nextPermissions = pruneExpiredPendingPermissions(
      pendingPermissionsRef.current,
    );
    if (nextPermissions !== pendingPermissionsRef.current) {
      setPendingPermissions(nextPermissions);
      return;
    }

    const nextTimeoutMs = getNextPendingPermissionTimeoutMs(
      pendingPermissionsRef.current,
    );
    if (nextTimeoutMs == null) {
      return;
    }

    const timeoutId = window.setTimeout(() => {
      setPendingPermissions((currentPermissions) =>
        pruneExpiredPendingPermissions(currentPermissions),
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

    await loadAgentSession(activeSessionKey, lifecycleContext, true);
  }, [lifecycleContext]);

  const loadOlderMessages = useCallback(async (): Promise<boolean> => {
    return loadOlderAgentConversationMessages({
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

  const loadRoundWindow = useCallback(async (roundId: string): Promise<boolean> => {
    return loadAgentConversationMessagesAroundRound({
      active_session_key_ref: activeSessionKeyRef,
      identity,
      history_cursor_ref: historyCursorRef,
      is_round_window_loading_ref: isRoundWindowLoadingRef,
      round_id: roundId,
      set_has_more_history: setHasMoreHistory,
      set_messages: setMessages,
      set_error: setError,
    });
  }, [
    identity,
    setError,
    setHasMoreHistory,
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
    setPendingAgentSlots(cancelRunningAgentSlots);
    setMessages((prev) =>
      reconcileStoppedSessionMessages(
        prev,
        runtimeSnapshotBeforeReset.terminalRoundIds,
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
        machine.syncRunningRounds(runningRoundIds);
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
        updateAssistantMessageStatus(prev, msgId, status),
      );
      setPendingAgentSlots((prev) =>
        updatePendingAgentSlotStatus(prev, msgId, status, roundId),
      );
      applyRuntimeTransition((machine) => {
        machine.updateMessageStatus(msgId, status, roundId);
      });
    },
    [applyRuntimeTransition, setMessages, setPendingAgentSlots],
  );

  const trackChatAck = useCallback(
    (ack: import("@/types").ChatAckData, _sessionKey?: string | null) => {
      applyRuntimeTransition((machine) => {
        machine.trackChatAck(ack);
      });
      clearPendingChatAck(ack.client_request_id);
      if (ack.client_message_id && ack.user_message_id) {
        setMessages((prev) =>
          replaceOptimisticUserMessage(
            prev,
            ack.client_message_id,
            ack.user_message_id,
            ack.round_id,
          ),
        );
      }
      setPendingAgentSlots((prev) => mergeChatAckPendingSlots(prev, ack));
    },
    [applyRuntimeTransition, clearPendingChatAck, setMessages, setPendingAgentSlots],
  );

  const trackAssistantMessage = useCallback(
    (message: AssistantMessage) => {
      applyRuntimeTransition((machine) => {
        machine.trackAssistantMessage(message);
      });
    },
    [applyRuntimeTransition],
  );

  const removeRewrittenRound = useCallback(
    (roundId: string) => {
      setMessages((prev) => removeRoundMessages(prev, roundId));
      setPendingPermissions((prev) =>
        filterRoundPendingPermissions(prev, roundId),
      );
      setPendingAgentSlots((prev) =>
        filterRoundPendingAgentSlots(prev, roundId),
      );
    },
    [setMessages, setPendingAgentSlots, setPendingPermissions],
  );

  const applyRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus) => {
      applyRuntimeTransition((machine) => {
        machine.trackRoundStatus(roundId, status);
      });

      if (status === "running") {
        return;
      }
      if (agentId && !runtimeMachineRef.current.snapshot().isLoading) {
        settleAgentWorkspaceWrites(agentId);
      }

      setPendingPermissions((prev) =>
        filterRoundPendingPermissions(prev, roundId),
      );
      setPendingAgentSlots((prev) =>
        filterRoundPendingAgentSlots(prev, roundId),
      );
      setMessages((prev) =>
        applyTerminalRoundMessageStatus(prev, roundId, status),
      );
    },
    [
      applyRuntimeTransition,
      agentId,
      settleAgentWorkspaceWrites,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
    ],
  );

  // Room slot 状态：只收口对应 agent slot，不结束 root turn。
  const applyAgentRoundStatus = useCallback(
    (payload: import("@/types").AgentRoundStatusEventPayload) => {
      if (!payload.is_terminal) {
        setPendingAgentSlots((prev) =>
          prev.map((slot) =>
            slot.agent_round_id === payload.agent_round_id
              ? { ...slot, status: "streaming" }
              : slot,
          ),
        );
        return;
      }
      setPendingAgentSlots((prev) =>
        filterAgentRoundPendingAgentSlots(prev, payload.agent_round_id),
      );
      setPendingPermissions((prev) =>
        prev.filter(
          (permission) =>
            permission.agent_round_id !== payload.agent_round_id,
        ),
      );
    },
    [setPendingAgentSlots, setPendingPermissions],
  );

  const handleWebsocketMessage = useCallback(
    (backendMessage: unknown) => {
      handleAgentConversationWebSocketMessage({
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
        apply_agent_round_status: applyAgentRoundStatus,
        track_chat_ack: trackChatAck,
        reject_chat_ack: rejectPendingChatAck,
        track_assistant_message: trackAssistantMessage,
        remove_rewritten_round: removeRewrittenRound,
        reload_current_session: reloadCurrentSession,
        settleAgentWorkspaceWrites: settleAgentWorkspaceWrites,
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
      applyAgentRoundStatus,
      settleAgentWorkspaceWrites,
      setPendingAgentSlots,
      setInputQueueItems,
      setMessages,
      setPendingPermissions,
      syncSessionStatus,
      rejectPendingChatAck,
      removeRewrittenRound,
      trackAssistantMessage,
      trackChatAck,
      updateMessageStatus,
    ],
  );

  useEffect(() => {
    runtimeMachineRef.current.setChatType(chatType);
    runtimeMachineRef.current.emit();
  }, [chatType]);

  const nextIdentityKey = getAgentConversationIdentityKey(identity);
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

  const { wsState, wsSend } = useAgentConversationSocket({
    wsUrl,
    agentId,
    roomId,
    conversationId,
    sessionKey,
    sessionSeqCursorRef,
    roomSeqCursorRef,
    wsSendRef,
    wsReconnectRef,
    wsStateRef,
    onMessage: handleWebsocketMessage,
    onError,
    setError,
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
      const request = await sendSessionMessage(content, actionContext, options);
      if (!request) {
        return;
      }

      applyRuntimeTransition((machine) => {
        machine.trackOutboundRequest(request.client_request_id);
      });

      try {
        await waitForChatAck(request.client_request_id, () => {
          failPendingChatAck(
            request.client_request_id,
            request.client_message_id,
            "消息未送达后端，请重试",
          );
        });
      } catch (error) {
        settleChatAckWaitFailure(
          request.client_request_id,
          request.client_message_id,
          error,
        );
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.clearOutboundRequest(request.client_request_id);
      });
    },
    [
      actionContext,
      applyRuntimeTransition,
      failPendingChatAck,
      settleChatAckWaitFailure,
      waitForChatAck,
    ],
  );

  const rewriteLastMessage = useCallback(
    async (targetRoundId: string, content: string) => {
      const request = await send_rewrite_last_user_message(targetRoundId, content, actionContext);
      if (!request) {
        return;
      }

      applyRuntimeTransition((machine) => {
        machine.trackOutboundRequest(request.client_request_id);
      });

      try {
        await waitForChatAck(request.client_request_id, () => {
          failPendingChatAck(
            request.client_request_id,
            request.client_message_id,
            "消息未送达后端，请重试",
          );
        });
      } catch (error) {
        settleChatAckWaitFailure(
          request.client_request_id,
          request.client_message_id,
          error,
        );
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.clearOutboundRequest(request.client_request_id);
      });
    },
    [
      actionContext,
      applyRuntimeTransition,
      failPendingChatAck,
      settleChatAckWaitFailure,
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
    (agentRoundId?: string) => {
      stopSessionGeneration(actionContext, agentRoundId);
      if (agentRoundId) {
        setPendingAgentSlots((prev) =>
          prev.map((slot) =>
            slot.agent_round_id === agentRoundId
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
    [actionContext, setPendingAgentSlots],
  );

  const sendPermissionResponse = useCallback(
    (payload: PermissionDecisionPayload) => {
      return sendSessionPermissionResponse(payload, actionContext);
    },
    [actionContext],
  );

  const startSession = useCallback(() => {
    cancelPendingChatAcks("会话已重建，未确认的消息发送已取消");
    startAgentSession(lifecycleContext);
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
      await loadAgentSession(id, lifecycleContext);
    },
    [lifecycleContext],
  );

  const clearSession = useCallback(() => {
    cancelPendingChatAcks("会话已清空，未确认的消息发送已取消");
    clearAgentSession(lifecycleContext);
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
      setSessionKey,
    ],
  );

  const resetSession = useCallback(() => {
    cancelPendingChatAcks("会话已重置，未确认的消息发送已取消");
    resetAgentSession(lifecycleContext);
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
    rewrite_last_user_message: rewriteLastMessage,
    enqueue_input_queue_message: enqueueInputQueueMessage,
    delete_input_queue_message: deleteInputQueueMessage,
    guide_input_queue_message: guideInputQueueMessage,
    reorder_input_queue_messages: reorderInputQueueMessages,
    bind_session_key: bindSessionKey,
    start_session: startSession,
    load_session: loadSession,
    load_older_messages: loadOlderMessages,
    load_round_window: loadRoundWindow,
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
