import { useMemo } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  AgentConversationIdentity,
  InputQueueItem,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";

import type { AgentConversationLifecycleContext } from "../conversation-lifecycle";
import type { AgentConversationHistoryCursor } from "../conversation-history";

interface UseAgentSessionLifecycleContextOptions {
  activeSessionKeyRef: RefObject<string | null>;
  backgroundMessagesRef: RefObject<Map<string, Message[]>>;
  historyCursorRef: RefObject<AgentConversationHistoryCursor>;
  identity: AgentConversationIdentity | null;
  loadRequestIdRef: RefObject<number>;
  reconcileRuntimeStateFromSnapshot: (messages: Message[]) => void;
  restoreVolatileSessionSnapshot: (sessionKey: string) => boolean;
  setError: Dispatch<SetStateAction<string | null>>;
  setHasMoreHistory: (nextValue: boolean) => void;
  setInputQueueItems: Dispatch<SetStateAction<InputQueueItem[]>>;
  setIsSessionLoading: Dispatch<SetStateAction<boolean>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
  setPendingPermissions: Dispatch<SetStateAction<PendingPermission[]>>;
  setSessionKey: Dispatch<SetStateAction<string | null>>;
}

export function useAgentSessionLifecycleContext({
  activeSessionKeyRef,
  backgroundMessagesRef,
  historyCursorRef,
  identity,
  loadRequestIdRef,
  reconcileRuntimeStateFromSnapshot,
  restoreVolatileSessionSnapshot,
  setError,
  setHasMoreHistory,
  setInputQueueItems,
  setIsSessionLoading,
  setMessages,
  setPendingAgentSlots,
  setPendingPermissions,
  setSessionKey,
}: UseAgentSessionLifecycleContextOptions): AgentConversationLifecycleContext {
  return useMemo<AgentConversationLifecycleContext>(() => ({
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
    restoreVolatileSessionSnapshot,
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
  }), [
    activeSessionKeyRef,
    backgroundMessagesRef,
    historyCursorRef,
    identity,
    loadRequestIdRef,
    reconcileRuntimeStateFromSnapshot,
    restoreVolatileSessionSnapshot,
    setError,
    setHasMoreHistory,
    setInputQueueItems,
    setIsSessionLoading,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    setSessionKey,
  ]);
}
