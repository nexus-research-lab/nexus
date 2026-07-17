import { useCallback, type Dispatch, type SetStateAction } from "react";

import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  RoundLifecycleStatus,
} from "@/types/conversation/message/event";
import type {
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { SessionStatusData } from "@/types/generated/protocol";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  applyTerminalRoundMessageStatus,
  cancelRunningAgentSlots,
  filterPendingSlotsFromSnapshot,
  filterRoundPendingAgentSlots,
  filterRoundPendingPermissions,
  mergeChatAckPendingSlots,
  reconcileAgentRoundPendingSlots,
  reconcileStoppedSessionMessages,
  removeRoundMessages,
  replaceOptimisticUserMessage,
  updateAssistantMessageStatus,
  updatePendingAgentSlotStatus,
} from "./model/conversation-runtime-reconciliation";
import { filterPendingPermissionsFromSnapshot } from "./model/pending-permission-model";
import { useConversationRuntimeMachine } from "./state/use-conversation-runtime-machine";
import { useConversationVolatileState } from "./state/use-conversation-volatile-state";

interface UseAgentConversationRuntimeParams {
  agentId: string | null;
  chatType: AgentConversationChatType;
  resolvePendingRequestAck: (clientRequestId?: string | null) => boolean;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  settleAgentWorkspaceWrites: (agentId: string) => void;
}

function getRunningRoundIds(payload: SessionStatusData): string[] {
  if (!Array.isArray(payload.running_round_ids)) {
    return [];
  }
  return payload.running_round_ids.filter(
    (roundId): roundId is string => typeof roundId === "string",
  );
}

/**
 * 编排运行状态机、易失交互状态与消息投影；具体状态规则由下层模型持有。
 */
export function useAgentConversationRuntime({
  agentId,
  chatType,
  resolvePendingRequestAck,
  setMessages,
  settleAgentWorkspaceWrites,
}: UseAgentConversationRuntimeParams) {
  const {
    clearOutboundRequest,
    isRoundTerminal,
    readSnapshot: readRuntimeSnapshot,
    reconcileFromSnapshot,
    reset: resetRuntimeMachine,
    setPendingPermissionCount,
    setRuntimeStatus,
    snapshot: runtimeSnapshot,
    syncRunningRounds,
    trackAssistantMessage,
    trackChatAck: trackRuntimeChatAck,
    trackOutboundRequest,
    trackRoundStatus,
    updateMessageStatus: updateRuntimeMessageStatus,
  } = useConversationRuntimeMachine(chatType);
  const {
    clearLiveState: clearLiveRuntimeState,
    pendingAgentSlots,
    pendingPermissions,
    readPendingAgentSlots,
    readPendingPermissions,
    setPendingAgentSlots,
    setPendingPermissions,
  } = useConversationVolatileState({
    onPendingPermissionCountChange: setPendingPermissionCount,
  });

  const reconcileRuntimeStateFromSnapshot = useCallback(
    (snapshotMessages: Message[]): void => {
      reconcileFromSnapshot(snapshotMessages);
      setPendingAgentSlots(filterPendingSlotsFromSnapshot(
        readPendingAgentSlots(),
        snapshotMessages,
        isRoundTerminal,
      ));
      setPendingPermissions(filterPendingPermissionsFromSnapshot(
        readPendingPermissions(),
        snapshotMessages,
        isRoundTerminal,
      ));
    },
    [
      isRoundTerminal,
      readPendingAgentSlots,
      readPendingPermissions,
      reconcileFromSnapshot,
      setPendingAgentSlots,
      setPendingPermissions,
    ],
  );

  const reconcileStoppedSession = useCallback((): void => {
    const snapshotBeforeReset = readRuntimeSnapshot();
    resetRuntimeMachine();
    if (agentId) {
      settleAgentWorkspaceWrites(agentId);
    }
    setPendingPermissions([]);
    setPendingAgentSlots(cancelRunningAgentSlots);
    setMessages((messages) => reconcileStoppedSessionMessages(
      messages,
      snapshotBeforeReset.terminalRoundIds,
    ));
  }, [
    agentId,
    readRuntimeSnapshot,
    resetRuntimeMachine,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    settleAgentWorkspaceWrites,
  ]);

  const syncSessionStatus = useCallback(
    (payload: SessionStatusData): void => {
      const runningRoundIds = getRunningRoundIds(payload);
      if (!payload.is_generating || runningRoundIds.length === 0) {
        reconcileStoppedSession();
        return;
      }
      syncRunningRounds(runningRoundIds);
    },
    [reconcileStoppedSession, syncRunningRounds],
  );

  const updateMessageStatus = useCallback(
    (
      messageId: string,
      status: AssistantMessageStatus,
      roundId?: string | null,
    ): void => {
      setMessages((messages) => updateAssistantMessageStatus(
        messages,
        messageId,
        status,
      ));
      setPendingAgentSlots((slots) => updatePendingAgentSlotStatus(
        slots,
        messageId,
        status,
        roundId,
      ));
      updateRuntimeMessageStatus(messageId, status, roundId);
    },
    [setMessages, setPendingAgentSlots, updateRuntimeMessageStatus],
  );

  const trackChatAck = useCallback((ack: ChatAckData): void => {
    trackRuntimeChatAck(ack);
    resolvePendingRequestAck(ack.client_request_id);
    if (ack.client_message_id && ack.user_message_id) {
      setMessages((messages) => replaceOptimisticUserMessage(
        messages,
        ack.client_message_id,
        ack.user_message_id,
        ack.round_id,
        ack.user_message_committed,
      ));
    }
    setPendingAgentSlots((slots) => mergeChatAckPendingSlots(slots, ack));
  }, [
    resolvePendingRequestAck,
    setMessages,
    setPendingAgentSlots,
    trackRuntimeChatAck,
  ]);

  const removeRewrittenRound = useCallback((roundId: string): void => {
    setMessages((messages) => removeRoundMessages(messages, roundId));
    setPendingPermissions((permissions) => (
      filterRoundPendingPermissions(permissions, roundId)
    ));
    setPendingAgentSlots((slots) => (
      filterRoundPendingAgentSlots(slots, roundId)
    ));
  }, [setMessages, setPendingAgentSlots, setPendingPermissions]);

  const applyRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus): void => {
      trackRoundStatus(roundId, status);
      if (status === "running") {
        return;
      }
      if (agentId && !readRuntimeSnapshot().isLoading) {
        settleAgentWorkspaceWrites(agentId);
      }
      setPendingPermissions((permissions) => (
        filterRoundPendingPermissions(permissions, roundId)
      ));
      setPendingAgentSlots((slots) => (
        filterRoundPendingAgentSlots(slots, roundId)
      ));
      setMessages((messages) => applyTerminalRoundMessageStatus(
        messages,
        roundId,
        status,
      ));
    },
    [
      agentId,
      readRuntimeSnapshot,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
      settleAgentWorkspaceWrites,
      trackRoundStatus,
    ],
  );

  const applyAgentRoundStatus = useCallback(
    (payload: AgentRoundStatusEventPayload): void => {
      setPendingAgentSlots((slots) => reconcileAgentRoundPendingSlots(
        slots,
        payload.agent_round_id,
        payload.is_terminal,
      ));
      if (!payload.is_terminal) {
        return;
      }
      setPendingPermissions((permissions) => permissions.filter(
        (permission) => permission.agent_round_id !== payload.agent_round_id,
      ));
    },
    [setPendingAgentSlots, setPendingPermissions],
  );

  return {
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
    setRuntimeStatus,
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    updateMessageStatus,
  };
}
