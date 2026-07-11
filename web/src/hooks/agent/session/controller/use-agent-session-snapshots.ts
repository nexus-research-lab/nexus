import { useCallback, useEffect, useRef } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message, RoomPendingAgentSlotState } from "@/types";

import {
  mergeLoadedMessages,
  upsertMessage,
} from "../../message/message-collection-model";
import {
  isEphemeralMessage,
  type AgentConversationRuntimeSnapshot,
} from "../../runtime/model/conversation-runtime-state";
import {
  buildVolatileConversationSnapshot,
  mergePendingAgentSlots,
} from "../../runtime/snapshot/conversation-volatile-model";
import {
  readVolatileConversationSnapshot,
  removeVolatileConversationSnapshot,
  writeVolatileConversationSnapshot,
} from "../../runtime/snapshot/conversation-volatile-storage";

interface UseAgentSessionSnapshotsOptions {
  messages: Message[];
  pendingAgentSlots: RoomPendingAgentSlotState[];
  reconcileRuntimeStateFromSnapshot: (messages: Message[]) => void;
  runtimeSnapshot: AgentConversationRuntimeSnapshot;
  sessionKey: string | null;
  setError: Dispatch<SetStateAction<string | null>>;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
}

export function useAgentSessionSnapshots({
  messages,
  pendingAgentSlots,
  reconcileRuntimeStateFromSnapshot,
  runtimeSnapshot,
  sessionKey,
  setError,
  setMessages,
  setPendingAgentSlots,
}: UseAgentSessionSnapshotsOptions): {
  backgroundMessagesRef: RefObject<Map<string, Message[]>>;
  onBackgroundMessage: (targetSessionKey: string, message: Message) => void;
  restoreVolatileSessionSnapshot: (targetSessionKey: string) => boolean;
} {
  const backgroundMessagesRef = useRef<Map<string, Message[]>>(new Map());

  const onBackgroundMessage = useCallback((
    targetSessionKey: string,
    message: Message,
  ): void => {
    if (isEphemeralMessage(message)) {
      return;
    }
    const currentMessages = backgroundMessagesRef.current.get(targetSessionKey)
      ?? [];
    backgroundMessagesRef.current.set(
      targetSessionKey,
      upsertMessage(currentMessages, message),
    );
  }, []);

  const restoreVolatileSessionSnapshot = useCallback((
    targetSessionKey: string,
  ): boolean => {
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
      restoredMessages.length > 0
      || snapshot.pending_agent_slots.length > 0
    );
  }, [
    reconcileRuntimeStateFromSnapshot,
    setError,
    setMessages,
    setPendingAgentSlots,
  ]);

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

  return {
    backgroundMessagesRef,
    onBackgroundMessage,
    restoreVolatileSessionSnapshot,
  };
}
