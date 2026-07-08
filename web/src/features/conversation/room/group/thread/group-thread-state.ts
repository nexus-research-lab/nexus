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
  roundId: string;
  agentId: string;
}

/** Thread 面板展示数据，由消费侧从 room-thread-live store 派生（见 use-room-thread-panel-data）。 */
export interface ThreadPanelData {
  messages: Message[];
  agentName: string | null;
  agentAvatar: string | null;
  userAvatar?: string | null;
  isLoading: boolean;
  pendingPermissions: PendingPermission[];
  onPermissionResponse?: (payload: PermissionDecisionPayload) => boolean;
  canRespondToPermissions?: boolean;
  permissionReadOnlyReason?: string;
  onStopMessage?: (msgId: string) => void;
  onOpenWorkspaceFile?: (path: string) => void;
}

interface ThreadControlState {
  activeThread: ThreadTarget | null;
  openThread: (roundId: string, agentId: string) => void;
  closeThread: () => void;
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
