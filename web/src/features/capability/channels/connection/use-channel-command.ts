import { useCallback, useRef, useState } from "react";

import type { ChannelPendingAction } from "./channel-connection-model";

export type RunChannelCommand = <Result>(
  action: ChannelPendingAction,
  command: () => Promise<Result>,
) => Promise<Result | undefined>;

export function useChannelCommand() {
  const pendingRef = useRef<ChannelPendingAction | null>(null);
  const [pendingAction, setPendingAction] =
    useState<ChannelPendingAction | null>(null);

  const runCommand = useCallback<RunChannelCommand>(async (action, command) => {
    // React 状态提交前仍可能发生重复点击，命令锁必须同步写入 ref。
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
