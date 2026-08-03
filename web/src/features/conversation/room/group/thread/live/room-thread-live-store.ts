import { create } from "zustand";

import type { Message } from "@/types/conversation/message/entity";
import type {
  RoomAgentExecutionState,
  RoomPendingAgentSlotState,
} from "@/types/agent/agent-conversation";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

export interface RoomThreadLiveSource {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  messageGroups: Map<string, Message[]>;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
  roomAgentExecutionStateGroups: Map<string, RoomAgentExecutionState[]>;
}

interface RoomThreadLiveState {
  clearSource: () => void;
  setSource: (source: RoomThreadLiveSource) => void;
  source: RoomThreadLiveSource | null;
}

export const useRoomThreadLiveStore = create<RoomThreadLiveState>()((set) => ({
  clearSource: () => set({ source: null }),
  setSource: (source) => set({ source }),
  source: null,
}));
