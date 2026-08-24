/**
 * INPUT: 当前/后台 Session 的可恢复消息与易失运行态。
 * OUTPUT: 受限后台消息缓存和按 Session 隔离的易失快照恢复。
 * POS: Session 切换时的非 canonical 浏览器恢复层。
 */
import { useCallback, useEffect, useRef } from "react";
import type { Dispatch, RefObject, SetStateAction } from "react";

import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";

import {
  mergeLoadedMessages,
  upsertMessage,
} from "../../message/message-collection-model";
import type { ConversationReliabilityController } from "../../reliability/use-conversation-reliability";
import { boundLoadedMessages } from "../../message/message-window-model";
import {
  isRecoverableMessage,
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
  chatType: "dm" | "group";
  messages: Message[];
  pendingAgentSlots: RoomPendingAgentSlotState[];
  reconcileRuntimeStateFromSnapshot: (messages: Message[]) => void;
  runtimeSnapshot: AgentConversationRuntimeSnapshot;
  sessionKey: string | null;
  reliability: ConversationReliabilityController;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  setPendingAgentSlots: Dispatch<SetStateAction<RoomPendingAgentSlotState[]>>;
}

export function useAgentSessionSnapshots({
  chatType,
  messages,
  pendingAgentSlots,
  reconcileRuntimeStateFromSnapshot,
  runtimeSnapshot,
  sessionKey,
  reliability,
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
    if (!isRecoverableMessage(message)) {
      return;
    }
    const currentMessages = backgroundMessagesRef.current.get(targetSessionKey)
      ?? [];
    backgroundMessagesRef.current.set(
      targetSessionKey,
      boundLoadedMessages(upsertMessage(currentMessages, message), {
        preference: "latest",
      }),
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
      restoredMessages = boundLoadedMessages(
        mergeLoadedMessages(snapshot.messages, currentMessages),
        { preference: "latest" },
      );
      return restoredMessages;
    });
    setPendingAgentSlots((currentSlots) => (
      mergePendingAgentSlots(snapshot.pending_agent_slots, currentSlots)
    ));
    reliability.reconcileSession(targetSessionKey, restoredMessages, chatType);
    reconcileRuntimeStateFromSnapshot(restoredMessages);
    return (
      restoredMessages.length > 0
      || snapshot.pending_agent_slots.length > 0
    );
  }, [
    chatType,
    reconcileRuntimeStateFromSnapshot,
    reliability,
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
