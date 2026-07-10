import {
  SetStateAction,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
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
  WebSocketMessage,
  WebSocketState,
} from "@/types";
import {
  InputQueueItem,
  RoomEventPayload,
  UseAgentConversationOptions,
  UseAgentConversationReturn,
  getAgentConversationIdentityKey,
} from "@/types/agent/agent-conversation";
import {
  loadAgentSession,
  type AgentConversationLifecycleContext,
} from "./conversation-lifecycle";
import {
  dedupeMessagesById,
  mergeLoadedMessages,
  upsertMessage,
} from "./message-helpers";
import { handleAgentConversationWebSocketMessage } from "./websocket-event-handler";
import type { AgentConversationActionContext } from "./conversation-actions";
import { removeFailedOutboundUserMessage } from "./conversation-runtime-reconciliation";
import {
  buildVolatileConversationSnapshot,
  isEphemeralMessage,
  mergePendingAgentSlots,
  readVolatileConversationSnapshot,
  removeVolatileConversationSnapshot,
  writeVolatileConversationSnapshot,
} from "./conversation-volatile-snapshot";
import { useConversationStreamBuffer } from "./use-conversation-stream-buffer";
import { usePendingChatAcks } from "./use-pending-chat-acks";
import { useAgentConversationActions } from "./use-agent-conversation-actions";
import { useAgentConversationHistory } from "./use-agent-conversation-history";
import { useAgentConversationRuntime } from "./use-agent-conversation-runtime";
import { useAgentConversationSession } from "./use-agent-conversation-session";
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
  const identitySessionKey = identity?.session_key?.trim() || null;

  const [messages, setMessagesState] = useState<Message[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [sessionKey, setSessionKey] = useResettableState<string | null>(identitySessionKey, identitySessionKey);
  const [isSessionLoading, setIsSessionLoading] = useState(false);
  const [inputQueueItems, setInputQueueItemsState] = useState<
    InputQueueItem[]
  >([]);

  const activeSessionKeyRef = useRef<string | null>(identitySessionKey);
  const activeIdentityKeyRef = useRef<string | null>(
    getAgentConversationIdentityKey(identity),
  );
  const loadRequestIdRef = useRef(0);
  const sessionSeqCursorRef = useRef(0);
  const roomSeqCursorRef = useRef(0);
  const wsSendRef = useRef<
    (payload: WebSocketMessage) => {
      disposition: "sent" | "queued" | "dropped";
    }
  >(() => ({ disposition: "dropped" }));
  const wsReconnectRef = useRef<() => void>(() => {});
  const wsStateRef = useRef<WebSocketState>("disconnected");
  // 非当前会话的完整消息先按 session 缓存，切换回来时再与服务端快照合并。
  const bgMessageCacheRef = useRef<Map<string, Message[]>>(new Map());
  const setMessages = useCallback((nextState: SetStateAction<Message[]>) => {
    setMessagesState((currentMessages) => {
      const nextMessages =
        typeof nextState === "function"
          ? nextState(currentMessages)
          : nextState;
      return dedupeMessagesById(nextMessages);
    });
  }, []);

  const {
    cancel_pending_chat_acks: cancelPendingChatAcks,
    clear_pending_chat_ack: clearPendingChatAck,
    reject_pending_chat_ack: rejectPendingChatAck,
    wait_for_chat_ack: waitForChatAck,
  } = usePendingChatAcks();

  const {
    applyAgentRoundStatus,
    applyRoundStatus,
    clearLiveRuntimeState,
    clearOutboundRequest,
    pendingAgentSlots,
    pendingPermissions,
    reconcileRuntimeStateFromSnapshot,
    removeRewrittenRound,
    resetRuntimeMachine,
    runtimeSnapshot,
    setPendingAgentSlots,
    setPendingPermissions,
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    updateMessageStatus,
  } = useAgentConversationRuntime({
    agentId,
    chatType,
    clearPendingChatAck,
    setMessages,
    settleAgentWorkspaceWrites,
  });
  const isLoading = runtimeSnapshot.isLoading;
  const runtimePhase = runtimeSnapshot.phase;
  const liveRoundIds = runtimeSnapshot.liveRoundIds;

  const {
    hasMoreHistory,
    historyCursorRef,
    historyPrependToken,
    isHistoryLoading,
    loadOlderMessages,
    loadRoundWindow,
    resetHistoryPagination,
    setHasMoreHistory,
  } = useAgentConversationHistory({
    activeSessionKeyRef,
    identity,
    setError,
    setMessages,
  });

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

  const clearLiveSessionState = useCallback(() => {
    clearLiveRuntimeState();
    setInputQueueItems((currentItems) =>
      currentItems.length ? [] : currentItems,
    );
  }, [
    clearLiveRuntimeState,
    setInputQueueItems,
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

  // 超时只负责拒绝 ACK 等待并触发重连，失败状态统一由 Promise catch 收口。
  const handleChatAckTimeout = useCallback(
    (clientRequestId: string, message: string) => {
      if (!rejectPendingChatAck(clientRequestId, message)) {
        return;
      }
      if (wsStateRef.current === "connected") {
        wsReconnectRef.current();
      }
    },
    [rejectPendingChatAck],
  );

  const settleChatAckWaitFailure = useCallback(
    (clientRequestId: string, clientMessageId: string, error: unknown) => {
      const message =
        error instanceof Error ? error.message : "消息未送达后端，请重试";
      clearOutboundRequest(clientRequestId);
      setMessages((prev) =>
        removeFailedOutboundUserMessage(prev, clientMessageId),
      );
      setError(message);
    },
    [
      clearOutboundRequest,
      setError,
      setMessages,
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
      historyCursorRef,
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

  const reloadCurrentSession = useCallback(async () => {
    const activeSessionKey = activeSessionKeyRef.current;
    if (!activeSessionKey) {
      return;
    }

    await loadAgentSession(activeSessionKey, lifecycleContext, true);
  }, [lifecycleContext]);

  const enqueueStreamPayload = useConversationStreamBuffer(setMessages);

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
      messages,
      set_error: setError,
      set_messages: setMessages,
      set_pending_permissions: setPendingPermissions,
    }),
    [
      identity,
      sessionKey,
      wsState,
      wsSend,
      pendingPermissions,
      messages,
      setError,
      setMessages,
      setPendingPermissions,
    ],
  );

  const {
    deleteQueueMessage: deleteInputQueueMessage,
    enqueueQueueMessage: enqueueInputQueueMessage,
    guideQueueMessage: guideInputQueueMessage,
    reorderQueueMessages: reorderInputQueueMessages,
    rewriteLastMessage,
    sendMessage,
    sendPermissionResponse,
    stopGeneration,
  } = useAgentConversationActions({
    actionContext,
    clearOutboundRequest,
    handleChatAckTimeout,
    setPendingAgentSlots,
    settleChatAckWaitFailure,
    trackOutboundRequest,
    waitForChatAck,
  });

  const {
    bindSessionKey,
    clearSession,
    loadSession,
    resetSession,
    startSession,
  } = useAgentConversationSession({
    activeSessionKeyRef,
    cancelPendingChatAcks,
    clearLiveSessionState,
    lifecycleContext,
    resetHistoryPagination,
    resetRuntimeMachine,
    setIsSessionLoading,
    setSessionKey,
  });

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
