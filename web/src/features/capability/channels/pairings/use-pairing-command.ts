import { useCallback, useRef, useState } from "react";

export interface PairingPendingAction {
  kind: "delete" | "update";
  pairingId: string;
}

export type RunPairingCommand = <Result>(
  action: PairingPendingAction,
  command: () => Promise<Result>,
) => Promise<Result | undefined>;

export function usePairingCommand() {
  const pendingRef = useRef<PairingPendingAction | null>(null);
  const [pendingAction, setPendingAction] =
    useState<PairingPendingAction | null>(null);

  const runCommand = useCallback<RunPairingCommand>(async (action, command) => {
    // 列表只允许一个写命令在途，避免同一配对被并发覆盖。
    if (pendingRef.current) {
      return undefined;
    }
    pendingRef.current = action;
    setPendingAction(action);
    try {
      return await command();
    } finally {
      if (pendingRef.current === action) {
        pendingRef.current = null;
        setPendingAction(null);
      }
    }
  }, []);

  return { pendingAction, runCommand };
}
