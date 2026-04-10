/**
 * =====================================================
 * @File   : room-thread-state.ts
 * @Date   : 2026-04-07 17:55
 * @Author : leemysw
 * 2026-04-07 17:55   Create
 * =====================================================
 */

import { createContext, useContext } from "react";

import { Message } from "@/types/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/permission";

interface ThreadTarget {
  round_id: string;
  agent_id: string;
  auto_close_on_finish?: boolean;
}

interface OpenThreadOptions {
  auto_close_on_finish?: boolean;
}

/** Thread 面板数据，由 RoomChatPanel 设置，由布局层读取后渲染。 */
export interface ThreadPanelData {
  messages: Message[];
  agent_name: string | null;
  is_loading: boolean;
  pending_permissions: PendingPermission[];
  on_permission_response?: (payload: PermissionDecisionPayload) => boolean;
  can_respond_to_permissions?: boolean;
  permission_read_only_reason?: string;
  on_stop_message?: (msg_id: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

interface ThreadControlState {
  active_thread: ThreadTarget | null;
  open_thread: (round_id: string, agent_id: string, options?: OpenThreadOptions) => void;
  close_thread: () => void;
}

interface ThreadDataState {
  thread_panel_data: ThreadPanelData | null;
}

interface ThreadDataDispatchState {
  set_thread_panel_data: (data: ThreadPanelData | null) => void;
}

export const ThreadControlContext = createContext<ThreadControlState | null>(null);
export const ThreadDataContext = createContext<ThreadDataState | null>(null);
export const ThreadDataDispatchContext = createContext<ThreadDataDispatchState | null>(null);

export function useRoomThread(): ThreadControlState {
  const context = useContext(ThreadControlContext);
  if (!context) {
    throw new Error("useRoomThread must be used within RoomThreadContextProvider");
  }
  return context;
}

export function useThreadPanelData(): ThreadDataState {
  const context = useContext(ThreadDataContext);
  if (!context) {
    throw new Error("useThreadPanelData must be used within RoomThreadContextProvider");
  }
  return context;
}

export function useSetThreadPanelData(): ThreadDataDispatchState {
  const context = useContext(ThreadDataDispatchContext);
  if (!context) {
    throw new Error("useSetThreadPanelData must be used within RoomThreadContextProvider");
  }
  return context;
}

export type {
  OpenThreadOptions,
  ThreadControlState,
  ThreadDataState,
  ThreadDataDispatchState,
  ThreadTarget,
};
