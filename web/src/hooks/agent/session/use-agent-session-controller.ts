import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import { areEquivalentSessionKeys } from "@/lib/conversation/session-key";
import type { Message, RoomPendingAgentSlotState } from "@/types";
import {
  getAgentConversationIdentityKey,
  type AgentConversationIdentity,
  type InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/permission";

import {
  mergeLoadedMessages,
  upsertMessage,
} from "../message/message-collection-model";
import type { AgentConversationRuntimeSnapshot } from "../runtime/agent-conversation-runtime-machine";
import {
  buildVolatileConversationSnapshot,
  isEphemeralMessage,
  mergePendingAgentSlots,
  readVolatileConversationSnapshot,
  removeVolatileConversationSnapshot,
  writeVolatileConversationSnapshot,
} from "../runtime/conversation-volatile-snapshot";
import {
  loadAgentSession,
  type AgentConversationLifecycleContext,
} from "./conversation-lifecycle";
import { useAgentConversationHistory } from "./use-agent-conversation-history";
import { useAgentConversationSession } from "./use-agent-conversation-session";

interface AgentSessionState {
  messages: Message[];
  pendingAgentSlots: RoomPendingAgentSlotState[];
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
}

interface AgentSessionRuntime {
  clearLiveRuntimeState: () => void;
  reconcileRuntimeStateFromSnapshot: (messages: Message[]) => void;
  resetRuntimeMachine: () => void;
  snapshot: AgentConversationRuntimeSnapshot;
}

interface UseAgentSessionControllerParams {
  cancelPendingChatAcks: (reason: string) => void;
  identity: AgentConversationIdentity | null;
  identitySessionKey: string | null;
  roomSeqCursorRef: RefObject<number>;
  sessionSeqCursorRef: RefObject<number>;
  runtime: AgentSessionRuntime;
  state: AgentSessionState;
}

/**
 * 会话控制器统一持有会话键、历史窗口、后台缓存和易失快照。
 * 身份切换只在 effect 中迁移，避免渲染阶段写入多个状态源。
 */
export function useAgentSessionController({
  cancelPendingChatAcks,
  identity,
  identitySessionKey,
  roomSeqCursorRef,
  runtime,
  sessionSeqCursorRef,
  state,
}: UseAgentSessionControllerParams) {
  const {
    clearLiveRuntimeState,
    reconcileRuntimeStateFromSnapshot,
    resetRuntimeMachine,
    snapshot: runtimeSnapshot,
  } = runtime;
  const {
    messages,
    pendingAgentSlots,
    setError,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
  } = state;
  const [sessionKey, setSessionKey] = useResettableState<string | null>(
    identitySessionKey,
    identitySessionKey,
  );
  const [isSessionLoading, setIsSessionLoading] = useState(false);
  const [inputQueueItems, setInputQueueItemsState] = useState<
    InputQueueItem[]
  >([]);
  const activeSessionKeyRef = useRef<string | null>(identitySessionKey);
  const activeIdentityKeyRef = useRef<string | null>(
    getAgentConversationIdentityKey(identity),
  );
  const loadRequestIdRef = useRef(0);
  const backgroundMessagesRef = useRef<Map<string, Message[]>>(new Map());

  const setInputQueueItems = useCallback(
    (nextState: SetStateAction<InputQueueItem[]>): void => {
      setInputQueueItemsState((currentItems) => (
        typeof nextState === "function"
          ? nextState(currentItems)
          : nextState
      ));
    },
    [],
  );

  const clearLiveSessionState = useCallback((): void => {
    clearLiveRuntimeState();
    setInputQueueItems((currentItems) => (
      currentItems.length > 0 ? [] : currentItems
    ));
  }, [clearLiveRuntimeState, setInputQueueItems]);

  const isCurrentSessionEvent = useCallback(
    (incomingSessionKey?: string | null): boolean => (
      Boolean(incomingSessionKey) && areEquivalentSessionKeys(
        activeSessionKeyRef.current,
        incomingSessionKey,
      )
    ),
    [],
  );

  const onBackgroundMessage = useCallback(
    (targetSessionKey: string, message: Message): void => {
      if (isEphemeralMessage(message)) {
        return;
      }
      const currentMessages =
        backgroundMessagesRef.current.get(targetSessionKey) ?? [];
      backgroundMessagesRef.current.set(
        targetSessionKey,
        upsertMessage(currentMessages, message),
      );
    },
    [],
  );

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

  const lifecycleContext = useMemo<AgentConversationLifecycleContext>(
    () => ({
      identity,
      refs: {
        activeSessionKey: activeSessionKeyRef,
        backgroundMessages: backgroundMessagesRef,
        loadRequestId: loadRequestIdRef,
      },
      state: {
        setError,
        setInputQueueItems,
        setIsSessionLoading,
        setMessages,
        setPendingAgentSlots,
        setPendingPermissions,
        setSessionKey,
      },
      restoreVolatileSessionSnapshot: (targetSessionKey) => {
        const snapshot = readVolatileConversationSnapshot(targetSessionKey);
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
        setPendingAgentSlots((currentSlots) => (
          mergePendingAgentSlots(snapshot.pending_agent_slots, currentSlots)
        ));
        setError(null);
        reconcileRuntimeStateFromSnapshot(restoredMessages);
        return (
          restoredMessages.length > 0 ||
          snapshot.pending_agent_slots.length > 0
        );
      },
      onSessionMessagesLoaded: (loadedMessages, meta) => {
        if (!meta.isReload) {
          historyCursorRef.current = {
            before_round_id: meta.nextBeforeRoundId,
            before_round_timestamp: meta.nextBeforeRoundTimestamp,
          };
          setHasMoreHistory(meta.hasMoreHistory);
        }
        reconcileRuntimeStateFromSnapshot(loadedMessages);
      },
    }),
    [
      historyCursorRef,
      identity,
      reconcileRuntimeStateFromSnapshot,
      setError,
      setHasMoreHistory,
      setInputQueueItems,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
      setSessionKey,
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
    if (snapshot) {
      writeVolatileConversationSnapshot(sessionKey, snapshot);
      return;
    }
    removeVolatileConversationSnapshot(sessionKey);
  }, [messages, pendingAgentSlots, runtimeSnapshot, sessionKey]);

  const reloadCurrentSession = useCallback(async (): Promise<void> => {
    const activeSessionKey = activeSessionKeyRef.current;
    if (activeSessionKey) {
      await loadAgentSession(activeSessionKey, lifecycleContext, true);
    }
  }, [lifecycleContext]);

  useEffect(() => {
    const nextIdentityKey = getAgentConversationIdentityKey(identity);
    if (activeIdentityKeyRef.current === nextIdentityKey) {
      return;
    }
    activeIdentityKeyRef.current = nextIdentityKey;
    sessionSeqCursorRef.current = 0;
    roomSeqCursorRef.current = 0;
    resetHistoryPagination();
    clearLiveSessionState();
    cancelPendingChatAcks("会话上下文已切换，未确认的消息发送已取消");
    resetRuntimeMachine();
  }, [
    cancelPendingChatAcks,
    clearLiveSessionState,
    identity,
    resetHistoryPagination,
    roomSeqCursorRef,
    resetRuntimeMachine,
    sessionSeqCursorRef,
  ]);

  useEffect(() => {
    activeSessionKeyRef.current = identitySessionKey;
  }, [identitySessionKey]);

  useEffect(() => () => {
    cancelPendingChatAcks("会话已卸载，未确认的消息发送已取消");
  }, [cancelPendingChatAcks]);

  const sessionActions = useAgentConversationSession({
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
    ...sessionActions,
    activeSessionKeyRef,
    hasMoreHistory,
    historyPrependToken,
    inputQueueItems,
    isCurrentSessionEvent,
    isHistoryLoading,
    isSessionLoading,
    loadOlderMessages,
    loadRoundWindow,
    onBackgroundMessage,
    reloadCurrentSession,
    sessionKey,
    setInputQueueItems,
  };
}
