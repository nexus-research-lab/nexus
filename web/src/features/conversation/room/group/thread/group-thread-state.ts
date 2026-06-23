/**
 * =====================================================
 * @File   : group-thread-state.ts
 * @Date   : 2026-04-07 17:55
 * @Author : leemysw
 * 2026-04-07 17:55   Create
 * =====================================================
 */

import { createContext, useContext } from "react";

import { Message } from "@/types/conversation/message";
import { PendingPermission, PermissionDecisionPayload } from "@/types/conversation/permission";

interface ThreadTarget {
  round_id: string;
  agent_id: string;
}

/** Thread 面板展示数据，由消费侧从 room-thread-live store 派生（见 use-room-thread-panel-data）。 */
export interface ThreadPanelData {
  messages: Message[];
  agent_name: string | null;
  agent_avatar: string | null;
  user_avatar?: string | null;
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
  open_thread: (round_id: string, agent_id: string) => void;
  close_thread: () => void;
}

export const ThreadControlContext = createContext<ThreadControlState | null>(null);

export function useGroupThread(): ThreadControlState {
  const context = useContext(ThreadControlContext);
  if (!context) {
    throw new Error("useGroupThread must be used within GroupThreadContextProvider");
  }
  return context;
}

export type {
  ThreadControlState,
  ThreadTarget,
};
