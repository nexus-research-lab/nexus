"use client";

import { useCallback, useMemo, useState, type ReactNode } from "react";

import {
  ThreadControlContext,
  type ThreadControlState,
  type ThreadTarget,
} from "./group-thread-state";

// 控制上下文只保存选择态；高频实时数据由 `live/` 发布，避免消息流重渲染整棵子树。

export function GroupThreadContextProvider({
  children,
  onOpenThread,
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
    setActiveThread(null);
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
