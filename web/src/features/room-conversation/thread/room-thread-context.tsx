"use client";

import { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import { Message } from "@/types/message";

interface ThreadTarget {
  round_id: string;
  agent_id: string;
  auto_close_on_finish?: boolean;
}

interface OpenThreadOptions {
  auto_close_on_finish?: boolean;
}

/** Thread 面板数据 — 由 RoomChatPanel 设置，由 Layout 读取用于渲染 ThreadDetailPanel */
export interface ThreadPanelData {
  messages: Message[];
  agent_name: string | null;
  is_loading: boolean;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

// ── Context 1: Thread 控制状态（被 RoomChatPanel / cards 消费） ──────────────

interface ThreadControlState {
  active_thread: ThreadTarget | null;
  open_thread: (round_id: string, agent_id: string, options?: OpenThreadOptions) => void;
  close_thread: () => void;
}

const ThreadControlContext = createContext<ThreadControlState | null>(null);

export function useRoomThread(): ThreadControlState {
  const ctx = useContext(ThreadControlContext);
  if (!ctx) throw new Error("useRoomThread must be used within RoomThreadContextProvider");
  return ctx;
}

// ── Context 2: Thread 面板数据（仅被 Layout Inspector 消费） ──────────────────
// 与控制状态分离，避免数据更新触发 RoomChatPanel 重渲染导致无限循环。

interface ThreadDataState {
  thread_panel_data: ThreadPanelData | null;
  set_thread_panel_data: (data: ThreadPanelData | null) => void;
}

const ThreadDataContext = createContext<ThreadDataState | null>(null);

export function useThreadPanelData(): ThreadDataState {
  const ctx = useContext(ThreadDataContext);
  if (!ctx) throw new Error("useThreadPanelData must be used within RoomThreadContextProvider");
  return ctx;
}

// ── Provider ─────────────────────────────────────────────────────────────────

export function RoomThreadContextProvider({ children }: { children: React.ReactNode }) {
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

  const open_thread = useCallback((round_id: string, agent_id: string, options?: OpenThreadOptions) => {
    set_active_thread({
      round_id,
      agent_id,
      auto_close_on_finish: options?.auto_close_on_finish ?? false,
    });
  }, []);

  const close_thread = useCallback(() => {
    set_active_thread(null);
  }, []);

  const control_value = useMemo<ThreadControlState>(
    () => ({ active_thread, open_thread, close_thread }),
    [active_thread, open_thread, close_thread],
  );

  const data_value = useMemo<ThreadDataState>(
    () => ({ thread_panel_data, set_thread_panel_data }),
    [thread_panel_data, set_thread_panel_data],
  );

  return (
    <ThreadDataContext.Provider value={data_value}>
      <ThreadControlContext.Provider value={control_value}>
        {children}
      </ThreadControlContext.Provider>
    </ThreadDataContext.Provider>
  );
}
