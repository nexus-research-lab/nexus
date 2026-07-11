import { useCallback, useRef, useState } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import { useResettableState } from "@/hooks/ui/use-resettable-state";
import type { Message, RoomPendingAgentSlotState } from "@/types";
import type {
  AgentConversationIdentity,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/permission";

import type { AgentConversationRuntimeSnapshot } from "../../runtime/model/conversation-runtime-state";
import { loadAgentSession } from "../conversation-lifecycle";
import { useAgentConversationHistory } from "../use-agent-conversation-history";
import { useAgentConversationSession } from "../use-agent-conversation-session";
import { useAgentSessionIdentity } from "./use-agent-session-identity";
import { useAgentSessionLifecycleContext } from "./use-agent-session-lifecycle-context";
import { useAgentSessionSnapshots } from "./use-agent-session-snapshots";

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
  const [inputQueueItems, setInputQueueItems] = useState<InputQueueItem[]>([]);
  const activeSessionKeyRef = useRef<string | null>(identitySessionKey);
  const loadRequestIdRef = useRef(0);
  const clearLiveSessionState = useCallback((): void => {
    clearLiveRuntimeState();
    setInputQueueItems((currentItems) => (
      currentItems.length > 0 ? [] : currentItems
    ));
  }, [clearLiveRuntimeState]);

  const history = useAgentConversationHistory({
    activeSessionKeyRef,
    identity,
    setError,
    setMessages,
  });
  const snapshots = useAgentSessionSnapshots({
    messages,
    pendingAgentSlots,
    reconcileRuntimeStateFromSnapshot,
    runtimeSnapshot,
    sessionKey,
    setError,
    setMessages,
    setPendingAgentSlots,
  });
  const { isCurrentSessionEvent } = useAgentSessionIdentity({
    activeSessionKeyRef,
    cancelPendingChatAcks,
    clearLiveSessionState,
    identity,
    identitySessionKey,
    resetHistoryPagination: history.resetHistoryPagination,
    resetRuntimeMachine,
    roomSeqCursorRef,
    sessionSeqCursorRef,
  });
  const lifecycleContext = useAgentSessionLifecycleContext({
    activeSessionKeyRef,
    backgroundMessagesRef: snapshots.backgroundMessagesRef,
    historyCursorRef: history.historyCursorRef,
    identity,
    loadRequestIdRef,
    reconcileRuntimeStateFromSnapshot,
    restoreVolatileSessionSnapshot:
      snapshots.restoreVolatileSessionSnapshot,
    setError,
    setHasMoreHistory: history.setHasMoreHistory,
    setInputQueueItems,
    setIsSessionLoading,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    setSessionKey,
  });
  const reloadCurrentSession = useCallback(async (): Promise<void> => {
    const activeSessionKey = activeSessionKeyRef.current;
    if (activeSessionKey) {
      await loadAgentSession(activeSessionKey, lifecycleContext, true);
    }
  }, [lifecycleContext]);
  const sessionActions = useAgentConversationSession({
    activeSessionKeyRef,
    cancelPendingChatAcks,
    clearLiveSessionState,
    lifecycleContext,
    resetHistoryPagination: history.resetHistoryPagination,
    resetRuntimeMachine,
    setIsSessionLoading,
    setSessionKey,
  });

  return {
    ...sessionActions,
    activeSessionKeyRef,
    hasMoreHistory: history.hasMoreHistory,
    historyPrependToken: history.historyPrependToken,
    inputQueueItems,
    isCurrentSessionEvent,
    isHistoryLoading: history.isHistoryLoading,
    isSessionLoading,
    loadOlderMessages: history.loadOlderMessages,
    loadRoundWindow: history.loadRoundWindow,
    onBackgroundMessage: snapshots.onBackgroundMessage,
    reloadCurrentSession,
    sessionKey,
    setInputQueueItems,
  };
}
