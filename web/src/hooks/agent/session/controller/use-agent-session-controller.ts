/**
 * INPUT: Agent 会话 identity、消息/运行态依赖和 Session 命令。
 * OUTPUT: 页面消费的 Session controller 与请求生命周期 refs。
 * POS: Agent conversation Session 子域的 React composition root。
 */
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  AgentConversationIdentity,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type { AgentConversationRuntimeSnapshot } from "../../runtime/model/conversation-runtime-state";
import { loadAgentSession } from "../conversation-lifecycle";
import {
  loadedMessageRoundIds,
  type LoadedMessageWindow,
} from "../../message/message-window-model";
import type { MessageWindowUpdater } from "../../message/use-agent-message-collection";
import { hasOlderHistory } from "../conversation-history-model";
import { useAgentConversationHistory } from "../use-agent-conversation-history";
import { useAgentConversationSession } from "../use-agent-conversation-session";
import { useAgentSessionIdentity } from "./use-agent-session-identity";
import { useAgentSessionLifecycleContext } from "./use-agent-session-lifecycle-context";
import { useAgentSessionSnapshots } from "./use-agent-session-snapshots";
import type { ConversationReliabilityController } from "../../reliability/use-conversation-reliability";

interface AgentSessionState {
  messageWindowRef: RefObject<LoadedMessageWindow>;
  messages: Message[];
  pendingAgentSlots: RoomPendingAgentSlotState[];
  reliability: ConversationReliabilityController;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
  updateMessageWindow: (updater: MessageWindowUpdater) => void;
}

interface AgentSessionRuntime {
  clearLiveRuntimeState: () => void;
  reconcileRuntimeStateFromSnapshot: (messages: Message[]) => void;
  resetRuntimeMachine: () => void;
  snapshot: AgentConversationRuntimeSnapshot;
}

interface UseAgentSessionControllerParams {
  cancelPendingRequestAcks: (
    reason: string,
    keepPreserved?: boolean,
  ) => void;
  identity: AgentConversationIdentity | null;
  identitySessionKey: string | null;
  roomSeqCursorRef: RefObject<number>;
  sessionSeqCursorRef: RefObject<number>;
  runtime: AgentSessionRuntime;
  state: AgentSessionState;
}

export function useAgentSessionController({
  cancelPendingRequestAcks,
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
    messageWindowRef,
    messages,
    pendingAgentSlots,
    reliability,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    updateMessageWindow,
  } = state;
  const [sessionKey, setSessionKey] = useResettableState<string | null>(
    identitySessionKey,
    identitySessionKey,
  );
  const [isSessionLoading, setIsSessionLoading] = useResettableState(
    Boolean(identitySessionKey),
    identitySessionKey,
  );
  const [inputQueueItems, setInputQueueItems] = useState<InputQueueItem[]>([]);
  const activeSessionKeyRef = useRef<string | null>(identitySessionKey);
  const loadAbortControllerRef = useRef<AbortController | null>(null);
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
    updateMessageWindow,
    windowRef: messageWindowRef,
  });
  const residentRoundIds = useMemo(
    () => loadedMessageRoundIds(messages),
    [messages],
  );
  const { adoptHistoryStart } = history;
  const residentHeadRoundId = residentRoundIds[0] ?? null;
  useEffect(() => {
    if (residentHeadRoundId && !isSessionLoading) {
      adoptHistoryStart(residentHeadRoundId);
    }
  }, [adoptHistoryStart, isSessionLoading, residentHeadRoundId]);
  const snapshots = useAgentSessionSnapshots({
    chatType: identity?.chat_type ?? "dm",
    messages,
    pendingAgentSlots,
    reconcileRuntimeStateFromSnapshot,
    runtimeSnapshot,
    sessionKey,
    reliability,
    setMessages,
    setPendingAgentSlots,
  });
  const { isCurrentSessionEvent } = useAgentSessionIdentity({
    activeSessionKeyRef,
    cancelPendingRequestAcks,
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
    commitHistoryCursor: history.commitHistoryCursor,
    identity,
    loadAbortControllerRef,
    loadRequestIdRef,
    reconcileRuntimeStateFromSnapshot,
    restoreVolatileSessionSnapshot:
      snapshots.restoreVolatileSessionSnapshot,
    reliability,
    setInputQueueItems,
    setIsSessionLoading,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    setSessionKey,
  });
  const reloadCurrentSession = useCallback(async (): Promise<Message[] | null> => {
    const activeSessionKey = activeSessionKeyRef.current;
    if (activeSessionKey) {
      return loadAgentSession(activeSessionKey, lifecycleContext, true);
    }
    return null;
  }, [lifecycleContext]);
  const sessionActions = useAgentConversationSession({
    activeSessionKeyRef,
    cancelPendingRequestAcks,
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
    hasMoreHistory: hasOlderHistory(
      history.historyStartRoundId,
      history.hasMoreHistory,
      residentRoundIds,
    ),
    inputQueueItems,
    isCurrentSessionEvent,
    isHistoryLoading: history.isHistoryLoading,
    isSessionLoading,
    loadNewerMessages: history.loadNewerMessages,
    loadOlderMessages: history.loadOlderMessages,
    loadRoundWindow: history.loadRoundWindow,
    onBackgroundMessage: snapshots.onBackgroundMessage,
    reloadCurrentSession,
    resolvedHistoryRoundIds: history.resolvedHistoryRoundIds,
    returnToLatestMessages: history.returnToLatestMessages,
    sessionKey,
    setInputQueueItems,
  };
}
