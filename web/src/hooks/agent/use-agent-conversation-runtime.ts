import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
  type Dispatch,
  type SetStateAction,
} from "react";

import type {
  AgentRoundStatusEventPayload,
  AssistantMessage,
  AssistantMessageStatus,
  ChatAckData,
  Message,
  RoomPendingAgentSlotState,
  RoundLifecycleStatus,
  SessionStatusEventPayload,
} from "@/types";
import type { AgentConversationChatType } from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/permission";

import { AgentConversationRuntimeMachine } from "./agent-conversation-runtime-machine";
import {
  applyTerminalRoundMessageStatus,
  cancelRunningAgentSlots,
  filterAgentRoundPendingAgentSlots,
  filterRoundPendingAgentSlots,
  filterRoundPendingPermissions,
  mergeChatAckPendingSlots,
  reconcileStoppedSessionMessages,
  removeRoundMessages,
  replaceOptimisticUserMessage,
  updateAssistantMessageStatus,
  updatePendingAgentSlotStatus,
} from "./conversation-runtime-reconciliation";
import {
  filterPendingPermissionsFromSnapshot,
  filterPendingSlotsFromSnapshot,
  getNextPendingPermissionTimeoutMs,
  pruneExpiredPendingPermissions,
} from "./conversation-volatile-snapshot";

interface UseAgentConversationRuntimeParams {
  agentId: string | null;
  chatType: AgentConversationChatType;
  clearPendingChatAck: (clientRequestId?: string | null) => boolean;
  setMessages: Dispatch<SetStateAction<Message[]>>;
  settleAgentWorkspaceWrites: (agentId: string) => void;
}

/**
 * 维护后端运行态在前端的唯一投影。
 * slot、权限与状态机必须一起迁移，避免任一终态只清理其中一份状态。
 */
export function useAgentConversationRuntime({
  agentId,
  chatType,
  clearPendingChatAck,
  setMessages,
  settleAgentWorkspaceWrites,
}: UseAgentConversationRuntimeParams) {
  const runtimeMachineRef = useRef(
    new AgentConversationRuntimeMachine(chatType),
  );
  const runtimeSnapshot = useSyncExternalStore(
    useCallback((listener) => runtimeMachineRef.current.subscribe(listener), []),
    useCallback(() => runtimeMachineRef.current.snapshot(), []),
  );
  const [pendingAgentSlots, setPendingAgentSlotsState] = useState<
    RoomPendingAgentSlotState[]
  >([]);
  const [pendingPermissions, setPendingPermissionsState] = useState<
    PendingPermission[]
  >([]);
  const pendingAgentSlotsRef = useRef<RoomPendingAgentSlotState[]>([]);
  const pendingPermissionsRef = useRef<PendingPermission[]>([]);

  const applyRuntimeTransition = useCallback(
    (transition: (machine: AgentConversationRuntimeMachine) => void): void => {
      transition(runtimeMachineRef.current);
      runtimeMachineRef.current.emit();
    },
    [],
  );

  const setPendingAgentSlots = useCallback(
    (nextState: SetStateAction<RoomPendingAgentSlotState[]>): void => {
      const next = typeof nextState === "function"
        ? nextState(pendingAgentSlotsRef.current)
        : nextState;
      pendingAgentSlotsRef.current = next;
      setPendingAgentSlotsState(next);
    },
    [],
  );

  const setPendingPermissions = useCallback(
    (nextState: SetStateAction<PendingPermission[]>): void => {
      const next = typeof nextState === "function"
        ? nextState(pendingPermissionsRef.current)
        : nextState;
      pendingPermissionsRef.current = next;
      applyRuntimeTransition((machine) => {
        machine.setPendingPermissionCount(next.length);
      });
      setPendingPermissionsState(next);
    },
    [applyRuntimeTransition],
  );

  const clearLiveRuntimeState = useCallback((): void => {
    setPendingAgentSlots((currentSlots) => (
      currentSlots.length > 0 ? [] : currentSlots
    ));
    setPendingPermissions((currentPermissions) => (
      currentPermissions.length > 0 ? [] : currentPermissions
    ));
  }, [setPendingAgentSlots, setPendingPermissions]);

  const trackOutboundRequest = useCallback(
    (clientRequestId: string): void => {
      applyRuntimeTransition((machine) => {
        machine.trackOutboundRequest(clientRequestId);
      });
    },
    [applyRuntimeTransition],
  );

  const clearOutboundRequest = useCallback(
    (clientRequestId: string): void => {
      applyRuntimeTransition((machine) => {
        machine.clearOutboundRequest(clientRequestId);
      });
    },
    [applyRuntimeTransition],
  );

  const resetRuntimeMachine = useCallback((): void => {
    applyRuntimeTransition((machine) => {
      machine.reset();
    });
  }, [applyRuntimeTransition]);

  const reconcileRuntimeStateFromSnapshot = useCallback(
    (snapshotMessages: Message[]): void => {
      applyRuntimeTransition((machine) => {
        machine.reconcileFromSnapshot(snapshotMessages);
      });
      const isRoundTerminal = (roundId: string): boolean => (
        runtimeMachineRef.current.isRoundTerminal(roundId)
      );
      setPendingAgentSlots(filterPendingSlotsFromSnapshot(
        pendingAgentSlotsRef.current,
        snapshotMessages,
        isRoundTerminal,
      ));
      setPendingPermissions(filterPendingPermissionsFromSnapshot(
        pendingPermissionsRef.current,
        snapshotMessages,
        isRoundTerminal,
      ));
    },
    [applyRuntimeTransition, setPendingAgentSlots, setPendingPermissions],
  );

  const reconcileStoppedSession = useCallback((): void => {
    const snapshotBeforeReset = runtimeMachineRef.current.snapshot();
    resetRuntimeMachine();
    if (agentId) {
      settleAgentWorkspaceWrites(agentId);
    }
    setPendingPermissions([]);
    setPendingAgentSlots(cancelRunningAgentSlots);
    setMessages((currentMessages) => reconcileStoppedSessionMessages(
      currentMessages,
      snapshotBeforeReset.terminalRoundIds,
      chatType,
    ));
  }, [
    agentId,
    chatType,
    resetRuntimeMachine,
    setMessages,
    setPendingAgentSlots,
    setPendingPermissions,
    settleAgentWorkspaceWrites,
  ]);

  const syncSessionStatus = useCallback(
    (payload: SessionStatusEventPayload): void => {
      const runningRoundIds = Array.isArray(payload.running_round_ids)
        ? payload.running_round_ids.filter(
            (roundId): roundId is string => typeof roundId === "string",
          )
        : [];
      if (!payload.is_generating || runningRoundIds.length === 0) {
        reconcileStoppedSession();
        return;
      }
      applyRuntimeTransition((machine) => {
        machine.syncRunningRounds(runningRoundIds);
      });
    },
    [applyRuntimeTransition, reconcileStoppedSession],
  );

  const updateMessageStatus = useCallback(
    (
      messageId: string,
      status: AssistantMessageStatus,
      roundId?: string | null,
    ): void => {
      setMessages((currentMessages) => updateAssistantMessageStatus(
        currentMessages,
        messageId,
        status,
      ));
      setPendingAgentSlots((currentSlots) => updatePendingAgentSlotStatus(
        currentSlots,
        messageId,
        status,
        roundId,
      ));
      applyRuntimeTransition((machine) => {
        machine.updateMessageStatus(messageId, status, roundId);
      });
    },
    [applyRuntimeTransition, setMessages, setPendingAgentSlots],
  );

  const trackChatAck = useCallback(
    (ack: ChatAckData): void => {
      applyRuntimeTransition((machine) => {
        machine.trackChatAck(ack);
      });
      clearPendingChatAck(ack.client_request_id);
      if (ack.client_message_id && ack.user_message_id) {
        setMessages((currentMessages) => replaceOptimisticUserMessage(
          currentMessages,
          ack.client_message_id,
          ack.user_message_id,
          ack.round_id,
        ));
      }
      setPendingAgentSlots((currentSlots) => (
        mergeChatAckPendingSlots(currentSlots, ack)
      ));
    },
    [
      applyRuntimeTransition,
      clearPendingChatAck,
      setMessages,
      setPendingAgentSlots,
    ],
  );

  const trackAssistantMessage = useCallback(
    (message: AssistantMessage): void => {
      applyRuntimeTransition((machine) => {
        machine.trackAssistantMessage(message);
      });
    },
    [applyRuntimeTransition],
  );

  const removeRewrittenRound = useCallback(
    (roundId: string): void => {
      setMessages((currentMessages) => removeRoundMessages(
        currentMessages,
        roundId,
      ));
      setPendingPermissions((currentPermissions) => (
        filterRoundPendingPermissions(currentPermissions, roundId)
      ));
      setPendingAgentSlots((currentSlots) => (
        filterRoundPendingAgentSlots(currentSlots, roundId)
      ));
    },
    [setMessages, setPendingAgentSlots, setPendingPermissions],
  );

  const applyRoundStatus = useCallback(
    (roundId: string, status: RoundLifecycleStatus): void => {
      applyRuntimeTransition((machine) => {
        machine.trackRoundStatus(roundId, status);
      });
      if (status === "running") {
        return;
      }
      if (agentId && !runtimeMachineRef.current.snapshot().isLoading) {
        settleAgentWorkspaceWrites(agentId);
      }
      setPendingPermissions((currentPermissions) => (
        filterRoundPendingPermissions(currentPermissions, roundId)
      ));
      setPendingAgentSlots((currentSlots) => (
        filterRoundPendingAgentSlots(currentSlots, roundId)
      ));
      setMessages((currentMessages) => applyTerminalRoundMessageStatus(
        currentMessages,
        roundId,
        status,
      ));
    },
    [
      agentId,
      applyRuntimeTransition,
      setMessages,
      setPendingAgentSlots,
      setPendingPermissions,
      settleAgentWorkspaceWrites,
    ],
  );

  const applyAgentRoundStatus = useCallback(
    (payload: AgentRoundStatusEventPayload): void => {
      if (!payload.is_terminal) {
        setPendingAgentSlots((currentSlots) => currentSlots.map((slot) => (
          slot.agent_round_id === payload.agent_round_id
            ? { ...slot, status: "streaming" }
            : slot
        )));
        return;
      }
      setPendingAgentSlots((currentSlots) => filterAgentRoundPendingAgentSlots(
        currentSlots,
        payload.agent_round_id,
      ));
      setPendingPermissions((currentPermissions) => currentPermissions.filter(
        (permission) => permission.agent_round_id !== payload.agent_round_id,
      ));
    },
    [setPendingAgentSlots, setPendingPermissions],
  );

  useEffect(() => {
    runtimeMachineRef.current.setChatType(chatType);
    runtimeMachineRef.current.emit();
  }, [chatType]);

  useEffect(() => {
    const nextPermissions = pruneExpiredPendingPermissions(
      pendingPermissionsRef.current,
    );
    if (nextPermissions !== pendingPermissionsRef.current) {
      setPendingPermissions(nextPermissions);
      return;
    }
    const nextTimeoutMs = getNextPendingPermissionTimeoutMs(
      pendingPermissionsRef.current,
    );
    if (nextTimeoutMs == null) {
      return;
    }
    const timeoutId = window.setTimeout(() => {
      setPendingPermissions((currentPermissions) => (
        pruneExpiredPendingPermissions(currentPermissions)
      ));
    }, nextTimeoutMs + 1);
    return () => {
      window.clearTimeout(timeoutId);
    };
  }, [pendingPermissions, setPendingPermissions]);

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
    syncSessionStatus,
    trackAssistantMessage,
    trackChatAck,
    trackOutboundRequest,
    updateMessageStatus,
  };
}
