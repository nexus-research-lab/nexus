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
  on_open_thread,
}: {
  children: ReactNode;
  on_open_thread?: () => void;
}) {
  const [active_thread, set_active_thread] = useState<ThreadTarget | null>(null);

  const open_thread = useCallback((round_id: string, agent_id: string) => {
    on_open_thread?.();
    set_active_thread((current) => (
      current?.round_id === round_id && current.agent_id === agent_id
        ? current
        : { round_id, agent_id }
    ));
  }, [on_open_thread]);

  const close_thread = useCallback(() => {
    set_active_thread((current) => current ? null : current);
  }, []);

  const control_value = useMemo<ThreadControlState>(
    () => ({ active_thread, open_thread, close_thread }),
    [active_thread, open_thread, close_thread],
  );

  return (
    <ThreadControlContext.Provider value={control_value}>
      {children}
    </ThreadControlContext.Provider>
  );
}
