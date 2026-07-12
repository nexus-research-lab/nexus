import { create } from "zustand";

import type { Message } from "@/types/conversation/message/entity";
import type { RoomPendingAgentSlotState } from "@/types/agent/agent-conversation";
import type {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/interaction/permission";

export interface RoomThreadLiveSource {
  agentAvatarMap: Record<string, string | null>;
  agentNameMap: Record<string, string>;
  currentUserAvatar: string | null;
  messageGroups: Map<string, Message[]>;
  onOpenWorkspaceFile?: (path: string) => void;
  onPermissionResponse: (payload: PermissionDecisionPayload) => boolean;
  onStopMessage: (msgId: string) => void;
  pendingPermissionGroups: Map<string, PendingPermission[]>;
  pendingSlotGroups: Map<string, RoomPendingAgentSlotState[]>;
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
