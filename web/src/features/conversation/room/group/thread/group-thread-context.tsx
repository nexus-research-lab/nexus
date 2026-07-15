/**
 * INPUT: Room Thread 子树、打开回调与精确执行轮目标。
 * OUTPUT: 仅保存当前 Thread 选择态的轻量 Context Provider。
 * POS: Room Thread 选择态的 React 生命周期边界。
 */
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

  const openThread = useCallback((
    roundId: string,
    agentId: string,
    agentRoundId: string | null = null,
  ) => {
    onOpenThread?.();
    setActiveThread((current) => (
      current?.roundId === roundId
        && current.agentId === agentId
        && current.agentRoundId === agentRoundId
        ? current
        : { roundId, agentId, agentRoundId }
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
