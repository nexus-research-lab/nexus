"use client";

import { ReactNode, useCallback, useMemo, useState } from "react";

import {
  ThreadControlContext,
  ThreadControlState,
  ThreadTarget,
} from "./group-thread-state";

// ── Provider ─────────────────────────────────────────────────────────────────
//
// 只负责 Thread 控制态（哪个 round/agent 的 thread 被打开）。
// 面板数据走 room-thread-live store（见 use-room-thread-panel-data.ts），不再经此。

export function GroupThreadContextProvider({
  children,
  onOpenThread: onOpenThread,
}: {
  children: ReactNode;
  onOpenThread?: () => void;
}) {
  const [activeThread, setActiveThread] = useState<ThreadTarget | null>(null);

  const openThread = useCallback((roundId: string, agentId: string) => {
    onOpenThread?.();
    setActiveThread((current) => (
      current?.roundId === roundId && current.agentId === agentId
        ? current
        : { roundId, agentId }
    ));
  }, [onOpenThread]);

  const closeThread = useCallback(() => {
    setActiveThread((current) => current ? null : current);
  }, []);

  const controlValue = useMemo<ThreadControlState>(
    () => ({ activeThread, openThread, closeThread }),
    [activeThread, openThread, closeThread],
  );

  return (
    <ThreadControlContext.Provider value={controlValue}>
      {children}
    </ThreadControlContext.Provider>
  );
}
