import {
  useCallback,
  useEffect,
  useRef,
  useSyncExternalStore,
} from "react";

import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { ChatAckData } from "@/types/conversation/message/event";
import type { RoundLifecycleStatus } from "@/types/conversation/message/event";
import type {
  AgentConversationChatType,
  AgentConversationRuntimeStatus,
} from "@/types/agent/agent-conversation";

import { AgentConversationRuntimeMachine } from "../model/agent-conversation-runtime-machine";

type RuntimeTransition = (machine: AgentConversationRuntimeMachine) => void;

/**
 * 隔离可变状态机，只向编排层暴露当前业务真正使用的命令。
 */
export function useConversationRuntimeMachine(
  chatType: AgentConversationChatType,
) {
  const machineRef = useRef(new AgentConversationRuntimeMachine(chatType));
  const subscribe = useCallback(
    (listener: () => void) => machineRef.current.subscribe(listener),
    [],
  );
  const readSnapshot = useCallback(() => machineRef.current.snapshot(), []);
  const snapshot = useSyncExternalStore(subscribe, readSnapshot);

  const transition = useCallback((apply: RuntimeTransition): void => {
    apply(machineRef.current);
    machineRef.current.emit();
  }, []);

  const clearOutboundRequest = useCallback(
    (clientRequestId: string): void => {
      transition((machine) => machine.clearOutboundRequest(clientRequestId));
    },
    [transition],
  );
  const isRoundTerminal = useCallback(
    (roundId: string): boolean => machineRef.current.isRoundTerminal(roundId),
    [],
  );
  const reconcileFromSnapshot = useCallback(
    (messages: Message[]): void => {
      transition((machine) => machine.reconcileFromSnapshot(messages));
    },
    [transition],
  );
  const reset = useCallback((): void => {
    transition((machine) => machine.reset());
  }, [transition]);
  const setPendingPermissionCount = useCallback(
    (count: number): void => {
      transition((machine) => machine.setPendingPermissionCount(count));
    },
    [transition],
  );
  const setRuntimeStatus = useCallback(
    (status: AgentConversationRuntimeStatus): void => {
      transition((machine) => machine.setRuntimeStatus(status));
    },
    [transition],
  );
  const syncRunningRounds = useCallback(
    (roundIds: string[]): void => {
      transition((machine) => machine.syncRunningRounds(roundIds));
    },
    [transition],
  );
  const trackAssistantMessage = useCallback(
    (message: AssistantMessage): void => {
      transition((machine) => machine.trackAssistantMessage(message));
    },
    [transition],
  );
  const trackChatAck = useCallback((ack: ChatAckData): void => {
    transition((machine) => machine.trackChatAck(ack));
  }, [transition]);
  const trackOutboundRequest = useCallback(
    (clientRequestId: string): void => {
      transition((machine) => machine.trackOutboundRequest(clientRequestId));
    },
    [transition],
  );
  const trackRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus): void => {
      transition((machine) => machine.trackRoundStatus(roundId, status));
    },
    [transition],
  );
  const updateMessageStatus = useCallback(
    (
      messageId: string,
      status: AssistantMessageStatus,
      roundId?: string | null,
    ): void => {
      transition((machine) => {
        machine.updateMessageStatus(messageId, status, roundId);
      });
    },
    [transition],
  );

  useEffect(() => {
    transition((machine) => machine.setChatType(chatType));
  }, [chatType, transition]);

  return {
    clearOutboundRequest,
    isRoundTerminal,
    readSnapshot,
    reconcileFromSnapshot,
    reset,
    setPendingPermissionCount,
    setRuntimeStatus,
    snapshot,
    syncRunningRounds,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    trackRoundStatus,
    updateMessageStatus,
  };
}
