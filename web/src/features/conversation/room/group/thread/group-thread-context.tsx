"use client";

import { ReactNode, useCallback, useMemo, useRef, useState } from "react";

import {
  ThreadControlContext,
  ThreadControlState,
  ThreadDataContext,
  ThreadDataDispatchContext,
  ThreadDataDispatchState,
  ThreadDataState,
  ThreadPanelData,
  ThreadTarget,
} from "./group-thread-state";

// ── Provider ─────────────────────────────────────────────────────────────────

export function GroupThreadContextProvider({children}: { children: ReactNode }) {
  // 控制状态
  const [active_thread, set_active_thread] = useState<ThreadTarget | null>(null);

  // 面板数据：用 ref 存储 + version counter 驱动重渲染
  const panel_data_ref = useRef<ThreadPanelData | null>(null);
  const [panel_data_version, set_panel_data_version] = useState(0);

  const set_thread_panel_data = useCallback((data: ThreadPanelData | null) => {
    const prev = panel_data_ref.current;
    // 浅比较关键字段：如果 messages 引用 + is_loading + agent_name 都没变，跳过
    if (data === prev) return;
    if (data && prev) {
      if (
        data.messages === prev.messages &&
        data.is_loading === prev.is_loading &&
        data.agent_name === prev.agent_name &&
        data.pending_permissions === prev.pending_permissions &&
        data.on_permission_response === prev.on_permission_response &&
        data.can_respond_to_permissions === prev.can_respond_to_permissions &&
        data.permission_read_only_reason === prev.permission_read_only_reason &&
        data.on_stop_message === prev.on_stop_message &&
        data.on_open_workspace_file === prev.on_open_workspace_file
      ) return;
    }
    panel_data_ref.current = data;
    set_panel_data_version((v) => v + 1);
  }, []);

  const thread_panel_data = useMemo(
    () => panel_data_ref.current,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [panel_data_version],
  );

  const open_thread = useCallback((round_id: string, agent_id: string) => {
    set_active_thread({
      round_id,
      agent_id,
    });
  }, []);

  const close_thread = useCallback(() => {
    set_active_thread(null);
  }, []);

  const control_value = useMemo<ThreadControlState>(
    () => ({active_thread, open_thread, close_thread}),
    [active_thread, open_thread, close_thread],
  );

  const data_value = useMemo<ThreadDataState>(
    () => ({thread_panel_data}),
    [thread_panel_data],
  );

  const data_dispatch_value = useMemo<ThreadDataDispatchState>(
    () => ({set_thread_panel_data}),
    [set_thread_panel_data],
  );

  return (
    <ThreadDataDispatchContext.Provider value={data_dispatch_value}>
      <ThreadDataContext.Provider value={data_value}>
        <ThreadControlContext.Provider value={control_value}>
          {children}
        </ThreadControlContext.Provider>
      </ThreadDataContext.Provider>
    </ThreadDataDispatchContext.Provider>
  );
}
