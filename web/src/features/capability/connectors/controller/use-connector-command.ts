import { useCallback, useRef, useState } from "react";

export type ConnectorPendingAction = {
  connectorId: string;
  kind:
    | "connect"
    | "connect-credential"
    | "disconnect"
    | "save-oauth-client"
    | "delete-oauth-client";
};

export type RunConnectorCommand = <Result>(
  action: ConnectorPendingAction,
  command: () => Promise<Result>,
) => Promise<Result | undefined>;

export function useConnectorCommand() {
  const pendingRef = useRef<ConnectorPendingAction | null>(null);
  const [pendingAction, setPendingAction] =
    useState<ConnectorPendingAction | null>(null);

  const runCommand = useCallback<RunConnectorCommand>(async (
    action,
    command,
  ) => {
    // 同一帧内的重复点击必须由 ref 拦截，不能等待 React 提交 busy 状态。
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
