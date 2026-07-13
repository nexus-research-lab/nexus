import { useCallback, useRef, useState } from "react";

export type ProviderPendingAction =
  | { kind: "save-provider" }
  | { kind: "toggle-provider" }
  | { kind: "delete-provider" }
  | { kind: "fetch-models" }
  | { kind: "add-model"; modelId: string }
  | { kind: "test-provider" }
  | { kind: "test-model"; modelId: string }
  | { kind: "toggle-model"; modelId: string }
  | { kind: "save-model-options"; modelId: string };

export type RunProviderCommand = <Result>(
  action: ProviderPendingAction,
  command: () => Promise<Result>,
) => Promise<Result | undefined>;

export function useProviderCommand() {
  const pendingRef = useRef<ProviderPendingAction | null>(null);
  const [pendingAction, setPendingAction] =
    useState<ProviderPendingAction | null>(null);

  const runCommand = useCallback<RunProviderCommand>(async (
    action,
    command,
  ) => {
    // ref 在事件同一帧内完成互斥，避免依赖尚未提交的 React 状态。
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
