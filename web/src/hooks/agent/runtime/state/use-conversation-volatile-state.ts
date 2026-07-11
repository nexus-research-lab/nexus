import {
  useCallback,
  useEffect,
  useRef,
  useState,
  type SetStateAction,
} from "react";

import type { RoomPendingAgentSlotState } from "@/types";
import type { PendingPermission } from "@/types/conversation/permission";

import {
  getNextPendingPermissionTimeoutMs,
  pruneExpiredPendingPermissions,
} from "../model/pending-permission-model";

interface UseConversationVolatileStateParams {
  onPendingPermissionCountChange: (count: number) => void;
}

function resolveStateAction<T>(next: SetStateAction<T>, current: T): T {
  return typeof next === "function"
    ? (next as (value: T) => T)(current)
    : next;
}

/**
 * slot 与权限需要同步读取最新值，ref 只作为事件回调的读模型，不是第二份状态源。
 */
export function useConversationVolatileState({
  onPendingPermissionCountChange,
}: UseConversationVolatileStateParams) {
  const [pendingAgentSlots, setPendingAgentSlotsState] = useState<
    RoomPendingAgentSlotState[]
  >([]);
  const [pendingPermissions, setPendingPermissionsState] = useState<
    PendingPermission[]
  >([]);
  const pendingAgentSlotsRef = useRef(pendingAgentSlots);
  const pendingPermissionsRef = useRef(pendingPermissions);

  const setPendingAgentSlots = useCallback(
    (nextState: SetStateAction<RoomPendingAgentSlotState[]>): void => {
      const next = resolveStateAction(nextState, pendingAgentSlotsRef.current);
      pendingAgentSlotsRef.current = next;
      setPendingAgentSlotsState(next);
    },
    [],
  );
  const setPendingPermissions = useCallback(
    (nextState: SetStateAction<PendingPermission[]>): void => {
      const next = resolveStateAction(nextState, pendingPermissionsRef.current);
      pendingPermissionsRef.current = next;
      onPendingPermissionCountChange(next.length);
      setPendingPermissionsState(next);
    },
    [onPendingPermissionCountChange],
  );
  const clearLiveState = useCallback((): void => {
    setPendingAgentSlots((slots) => slots.length > 0 ? [] : slots);
    setPendingPermissions((permissions) => (
      permissions.length > 0 ? [] : permissions
    ));
  }, [setPendingAgentSlots, setPendingPermissions]);
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
    clearLiveState,
    pendingAgentSlots,
    pendingPermissions,
    readPendingAgentSlots,
    readPendingPermissions,
    setPendingAgentSlots,
    setPendingPermissions,
  };
}
