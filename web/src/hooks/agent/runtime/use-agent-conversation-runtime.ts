/**
 * INPUT: 会话 runtime/interrupt ACK 事件、消息集合与易失 Room 增量 slot/权威 slot snapshot/权限/execution/stopping 状态。
 * OUTPUT: 单调运行快照、消息状态、首次展示锚点，以及由 stopping/权限/slot/空快照平滑接棒到终态结果的协调动作。
 * POS: transport 事件和纯 reconciliation 模型之间的 React 编排边界。
 */
import { useCallback, type Dispatch, type SetStateAction } from "react";

import type {
  AgentRoundStatusEventPayload,
  ChatAckData,
  RoundLifecycleStatus,
  StreamMessage,
} from "@/types/conversation/message/event";
import type {
  AssistantMessage,
  AssistantMessageStatus,
  Message,
} from "@/types/conversation/message/entity";
import type { SessionStatusData } from "@/types/generated/protocol";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";

import {
  applyTerminalAgentRoundMessageStatus,
  applyTerminalRoundMessageStatus,
  cancelRunningAgentSlots,
  filterPendingSlotsFromSnapshot,
  filterRoundPendingAgentSlots,
  filterRoundPendingPermissions,
  mergeChatAckPendingSlots,
  reconcileAgentRoundPendingSlots,
  reconcilePendingSlotsWithAssistantMessage,
  reconcileStoppedSessionMessages,
  removeRoundMessages,
  replaceOptimisticUserMessage,
  updateAssistantMessageStatus,
  updatePendingAgentSlotStatus,
} from "./model/conversation-runtime-reconciliation";
import { filterPendingPermissionsFromSnapshot } from "./model/pending-permission-model";
import {
  applyRoomAgentExecutionStatus,
  applyRoomExecutionRootStatus,
  confirmRoomAgentExecutionStop,
  removeRoomAgentExecutionRound,
  syncRoomAgentExecutionFromLiveMessage,
  stopRoomAgentExecutions,
  syncRoomAgentExecutionFromStream,
  syncRoomAgentExecutionsFromMessages,
} from "./model/room-agent-execution-state";
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
 * 编排运行状态机、易失交互状态与消息投影；终态 Room slot 等消息接棒后再清理。
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
    trackAssistantMessage: trackRuntimeAssistantMessage,
    trackChatAck: trackRuntimeChatAck,
    trackOutboundRequest,
    trackRoundStatus,
    updateMessageStatus: updateRuntimeMessageStatus,
  } = useConversationRuntimeMachine(chatType);
  const {
    acknowledgePermissionRequest,
    beginAgentRoundStop,
    clearLiveState: clearLiveRuntimeState,
    pendingAgentSlots,
    pendingPermissions,
    roomAgentExecutionStates,
    readPendingAgentSlots,
    readPendingPermissions,
    readStoppingAgentRoundIds,
    reconcilePendingAgentSlotSnapshot,
    setPendingAgentSlots,
    setPendingPermissions,
    setRoomAgentExecutionStates,
    settleAgentRoundStop,
    stoppingAgentRoundIds,
  } = useConversationVolatileState({
    onPendingPermissionCountChange: setPendingPermissionCount,
    trackRoomAgentExecutions: chatType === "group",
  });

  const reconcileRuntimeStateFromSnapshot = useCallback(
    (snapshotMessages: Message[]): void => {
      reconcileFromSnapshot(snapshotMessages);
      if (chatType === "group") {
        setRoomAgentExecutionStates((states) => (
          syncRoomAgentExecutionsFromMessages(states, snapshotMessages)
        ));
      }
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
      chatType,
      isRoundTerminal,
      readPendingAgentSlots,
      readPendingPermissions,
      reconcileFromSnapshot,
      setPendingAgentSlots,
      setPendingPermissions,
      setRoomAgentExecutionStates,
    ],
  );

  const reconcileStoppedSession = useCallback((): void => {
    const snapshotBeforeReset = readRuntimeSnapshot();
    resetRuntimeMachine();
    if (agentId) {
      settleAgentWorkspaceWrites(agentId);
    }
    setRoomAgentExecutionStates(stopRoomAgentExecutions);
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
    setRoomAgentExecutionStates,
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

  const trackAssistantMessage = useCallback(
    (message: AssistantMessage): void => {
      trackRuntimeAssistantMessage(message);
      if (chatType === "group") {
        setRoomAgentExecutionStates((states) => (
          syncRoomAgentExecutionFromLiveMessage(states, message)
        ));
      }
      setPendingAgentSlots((slots) => (
        reconcilePendingSlotsWithAssistantMessage(slots, message)
      ));
    },
    [
      chatType,
      setPendingAgentSlots,
      setRoomAgentExecutionStates,
      trackRuntimeAssistantMessage,
    ],
  );

  const trackStreamExecution = useCallback((stream: StreamMessage): void => {
    if (chatType !== "group") {
      return;
    }
    setRoomAgentExecutionStates((states) => (
      syncRoomAgentExecutionFromStream(states, stream)
    ));
  }, [chatType, setRoomAgentExecutionStates]);

  const trackChatAck = useCallback((ack: ChatAckData): void => {
    trackRuntimeChatAck(ack);
    if (ack.client_request_id) {
      resolvePendingRequestAck(ack.client_request_id);
    }
    if (ack.client_message_id && ack.user_message_id) {
      setMessages((messages) => replaceOptimisticUserMessage(
        messages,
        ack.client_message_id,
        ack.user_message_id,
        ack.round_id,
        ack.user_message_committed,
        ack.user_message_delivery_mode,
      ));
    }
    if (ack.pending_snapshot) {
      reconcilePendingAgentSlotSnapshot(mergeChatAckPendingSlots([], ack));
      return;
    }
    setPendingAgentSlots((slots) => mergeChatAckPendingSlots(slots, ack));
  }, [
    reconcilePendingAgentSlotSnapshot,
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
    setRoomAgentExecutionStates((states) => (
      removeRoomAgentExecutionRound(states, roundId)
    ));
  }, [
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    setRoomAgentExecutionStates,
  ]);

  const applyRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus): void => {
      trackRoundStatus(roundId, status);
      if (status === "running") {
        return;
      }
      setRoomAgentExecutionStates((states) => (
        applyRoomExecutionRootStatus(states, roundId, status)
      ));
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
      setRoomAgentExecutionStates,
      settleAgentWorkspaceWrites,
      trackRoundStatus,
    ],
  );

  const applyAgentRoundStatus = useCallback(
    (payload: AgentRoundStatusEventPayload): void => {
      setRoomAgentExecutionStates((states) => (
        applyRoomAgentExecutionStatus(states, payload)
      ));
      setPendingAgentSlots((slots) => reconcileAgentRoundPendingSlots(
        slots,
        payload.agent_round_id,
        payload.status,
      ));
      if (!payload.is_terminal) {
        return;
      }
      settleAgentRoundStop(payload.agent_round_id);
      setMessages((messages) => applyTerminalAgentRoundMessageStatus(
        messages,
        payload.agent_round_id,
        payload.status,
      ));
      setPendingPermissions((permissions) => permissions.filter(
        (permission) => permission.agent_round_id !== payload.agent_round_id,
      ));
    },
    [
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
      setRoomAgentExecutionStates,
      settleAgentRoundStop,
    ],
  );

  const confirmAgentRoundStop = useCallback((agentRoundId: string): void => {
    const normalizedAgentRoundId = agentRoundId.trim();
    if (!normalizedAgentRoundId) {
      return;
    }
    setRoomAgentExecutionStates((states) => (
      confirmRoomAgentExecutionStop(states, normalizedAgentRoundId)
    ));
    setPendingAgentSlots((slots) => reconcileAgentRoundPendingSlots(
      slots,
      normalizedAgentRoundId,
      "interrupted",
    ));
    setMessages((messages) => applyTerminalAgentRoundMessageStatus(
      messages,
      normalizedAgentRoundId,
      "interrupted",
    ));
    setPendingPermissions((permissions) => permissions.filter(
      (permission) => permission.agent_round_id !== normalizedAgentRoundId,
    ));
    settleAgentRoundStop(normalizedAgentRoundId);
  }, [
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    setRoomAgentExecutionStates,
    settleAgentRoundStop,
  ]);

  return {
    acknowledgePermissionRequest,
    applyAgentRoundStatus,
    applyRoundStatus,
    beginAgentRoundStop,
    clearLiveRuntimeState,
    clearOutboundRequest,
    confirmAgentRoundStop,
    pendingAgentSlots,
    pendingPermissions,
    readStoppingAgentRoundIds,
    roomAgentExecutionStates,
    reconcileRuntimeStateFromSnapshot,
    removeRewrittenRound,
    resetRuntimeMachine,
    runtimeSnapshot,
    setPendingAgentSlots,
    setPendingPermissions,
    setRuntimeStatus,
    settleAgentRoundStop,
    stoppingAgentRoundIds,
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    trackStreamExecution,
    updateMessageStatus,
  };
}
