/**
 * INPUT: Room 增量 slot/权威 slot snapshot/permission/精确停止动作、execution 跟踪模式与权限过期时钟。
 * OUTPUT: 同步可读的易失 slot/permission/execution/stopping 切片、快照缺失 execution 收口、拒绝 terminal execution 的迟到精确权限与 Session 清理命令。
 * POS: runtime 易失状态的 React owner；业务迁移委托给相邻纯 model。
 */
import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type SetStateAction,
} from "react";

import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type { PendingPermission } from "@/types/conversation/interaction/permission";
import {
  filterPendingPermissionsForTerminalRoomExecutions,
} from "@/lib/conversation/pending-permission-match";

import {
  getNextPendingPermissionTimeoutMs,
  pruneExpiredPendingPermissions,
} from "../model/pending-permission-model";
import {
  acknowledgeRoomAgentExecutionPermission,
  reconcileRoomAgentExecutionsFromSlotSnapshot,
  syncRoomAgentExecutionsFromPermissions,
  syncRoomAgentExecutionsFromSlots,
} from "../model/room-agent-execution-state";

interface UseConversationVolatileStateParams {
  onPendingPermissionCountChange: (count: number) => void;
  trackRoomAgentExecutions: boolean;
}

function resolveStateAction<T>(next: SetStateAction<T>, current: T): T {
  return typeof next === "function"
    ? (next as (value: T) => T)(current)
    : next;
}

export function addStoppingAgentRoundId(
  current: string[],
  agentRoundId: string,
): string[] {
  const normalizedAgentRoundId = agentRoundId.trim();
  if (!normalizedAgentRoundId || current.includes(normalizedAgentRoundId)) {
    return current;
  }
  return [...current, normalizedAgentRoundId];
}

export function removeStoppingAgentRoundId(
  current: string[],
  agentRoundId: string,
): string[] {
  const normalizedAgentRoundId = agentRoundId.trim();
  if (!normalizedAgentRoundId) {
    return current;
  }
  const next = current.filter((value) => value !== normalizedAgentRoundId);
  return next.length === current.length ? current : next;
}

/**
 * slot 与权限需要同步读取最新值，ref 只作为事件回调的读模型，不是第二份状态源。
 */
export function useConversationVolatileState({
  onPendingPermissionCountChange,
  trackRoomAgentExecutions,
}: UseConversationVolatileStateParams) {
  const [pendingAgentSlots, setPendingAgentSlotsState] = useState<
    RoomPendingAgentSlotState[]
  >([]);
  const [pendingPermissions, setPendingPermissionsState] = useState<
    PendingPermission[]
  >([]);
  const [roomAgentExecutionStates, setRoomAgentExecutionStatesState] = useState<
    RoomAgentExecutionState[]
  >([]);
  const [stoppingAgentRoundIds, setStoppingAgentRoundIdsState] = useState<
    string[]
  >([]);
  const pendingAgentSlotsRef = useRef(pendingAgentSlots);
  const pendingPermissionsRef = useRef(pendingPermissions);
  const roomAgentExecutionStatesRef = useRef(roomAgentExecutionStates);
  const stoppingAgentRoundIdsRef = useRef(stoppingAgentRoundIds);

  const setStoppingAgentRoundIds = useCallback(
    (nextState: SetStateAction<string[]>): void => {
      const next = resolveStateAction(
        nextState,
        stoppingAgentRoundIdsRef.current,
      );
      stoppingAgentRoundIdsRef.current = next;
      setStoppingAgentRoundIdsState(next);
    },
    [],
  );
  const beginAgentRoundStop = useCallback((agentRoundId: string): boolean => {
    const normalizedAgentRoundId = agentRoundId.trim();
    if (
      !normalizedAgentRoundId
      || stoppingAgentRoundIdsRef.current.includes(normalizedAgentRoundId)
    ) {
      return false;
    }
    setStoppingAgentRoundIds((current) => addStoppingAgentRoundId(
      current,
      normalizedAgentRoundId,
    ));
    return true;
  }, [setStoppingAgentRoundIds]);
  const settleAgentRoundStop = useCallback((agentRoundId: string): void => {
    const normalizedAgentRoundId = agentRoundId.trim();
    if (!normalizedAgentRoundId) {
      return;
    }
    setStoppingAgentRoundIds((current) => removeStoppingAgentRoundId(
      current,
      normalizedAgentRoundId,
    ));
  }, [setStoppingAgentRoundIds]);
  const readStoppingAgentRoundIds = useCallback(
    () => stoppingAgentRoundIdsRef.current,
    [],
  );

  const setRoomAgentExecutionStates = useCallback(
    (nextState: SetStateAction<RoomAgentExecutionState[]>): void => {
      const next = resolveStateAction(
        nextState,
        roomAgentExecutionStatesRef.current,
      );
      roomAgentExecutionStatesRef.current = next;
      if (trackRoomAgentExecutions) {
        const nextPermissions =
          filterPendingPermissionsForTerminalRoomExecutions(
            pendingPermissionsRef.current,
            next,
          );
        if (nextPermissions !== pendingPermissionsRef.current) {
          pendingPermissionsRef.current = nextPermissions;
          onPendingPermissionCountChange(nextPermissions.length);
          setPendingPermissionsState(nextPermissions);
        }
      }
      setRoomAgentExecutionStatesState(next);
    },
    [onPendingPermissionCountChange, trackRoomAgentExecutions],
  );
  const setPendingAgentSlots = useCallback(
    (nextState: SetStateAction<RoomPendingAgentSlotState[]>): void => {
      const next = resolveStateAction(nextState, pendingAgentSlotsRef.current);
      pendingAgentSlotsRef.current = next;
      if (trackRoomAgentExecutions) {
        setRoomAgentExecutionStates((states) => (
          syncRoomAgentExecutionsFromSlots(states, next)
        ));
      }
      setPendingAgentSlotsState(next);
    },
    [setRoomAgentExecutionStates, trackRoomAgentExecutions],
  );
  const reconcilePendingAgentSlotSnapshot = useCallback((
    slots: RoomPendingAgentSlotState[],
  ): void => {
    pendingAgentSlotsRef.current = slots;
    if (trackRoomAgentExecutions) {
      setRoomAgentExecutionStates((states) => (
        reconcileRoomAgentExecutionsFromSlotSnapshot(states, slots)
      ));
    }
    setPendingAgentSlotsState(slots);
  }, [setRoomAgentExecutionStates, trackRoomAgentExecutions]);
  const setPendingPermissions = useCallback(
    (nextState: SetStateAction<PendingPermission[]>): void => {
      const proposed = resolveStateAction(
        nextState,
        pendingPermissionsRef.current,
      );
      const next = trackRoomAgentExecutions
        ? filterPendingPermissionsForTerminalRoomExecutions(
            proposed,
            roomAgentExecutionStatesRef.current,
          )
        : proposed;
      pendingPermissionsRef.current = next;
      if (trackRoomAgentExecutions) {
        setRoomAgentExecutionStates((states) => (
          syncRoomAgentExecutionsFromPermissions(states, next)
        ));
      }
      onPendingPermissionCountChange(next.length);
      setPendingPermissionsState(next);
    },
    [
      onPendingPermissionCountChange,
      setRoomAgentExecutionStates,
      trackRoomAgentExecutions,
    ],
  );
  const acknowledgePermissionRequest = useCallback((requestId: string): void => {
    if (!trackRoomAgentExecutions) {
      return;
    }
    const permission = pendingPermissionsRef.current.find(
      (candidate) => candidate.request_id === requestId,
    );
    if (!permission) {
      return;
    }
    setRoomAgentExecutionStates((states) => (
      acknowledgeRoomAgentExecutionPermission(states, permission)
    ));
  }, [setRoomAgentExecutionStates, trackRoomAgentExecutions]);
  const clearLiveState = useCallback((): void => {
    setPendingAgentSlots((slots) => slots.length > 0 ? [] : slots);
    setPendingPermissions((permissions) => (
      permissions.length > 0 ? [] : permissions
    ));
    setRoomAgentExecutionStates((states) => states.length > 0 ? [] : states);
    setStoppingAgentRoundIds((ids) => ids.length > 0 ? [] : ids);
  }, [
    setPendingAgentSlots,
    setPendingPermissions,
    setRoomAgentExecutionStates,
    setStoppingAgentRoundIds,
  ]);
  const readPendingAgentSlots = useCallback(
    () => pendingAgentSlotsRef.current,
    [],
  );
  const readPendingPermissions = useCallback(
    () => pendingPermissionsRef.current,
    [],
  );

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
      setPendingPermissions(pruneExpiredPendingPermissions);
    }, nextTimeoutMs + 1);
    return () => window.clearTimeout(timeoutId);
  }, [pendingPermissions, setPendingPermissions]);

  return {
    acknowledgePermissionRequest,
    beginAgentRoundStop,
    clearLiveState,
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
  };
}
