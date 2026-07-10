/**
 * Room Thread Live Store
 *
 * [INPUT]: 依赖 zustand，依赖 @/types/conversation
 * [OUTPUT]: 对外提供 useRoomThreadLiveStore
 * [POS]: store 层的 Room Thread 实时切片；由 GroupChatPanel 发布，Thread 面板订阅
 * [PROTOCOL]: 变更时更新此头部，然后检查 CLAUDE.md
 *
 * 取代原 group-thread-context 的数据桥（ref + version + 深比较守卫 + push effect）。
 * 生产者发布切片但不订阅本 store → 写入不会重渲染生产者 → 结构上无反馈环。
 * Thread 面板经 selector 精确订阅 source，在自己 render 里派生展示数据。
 */

import { create } from "zustand";

import { Message, RoomPendingAgentSlotState } from "@/types/conversation/message";
import {
  PendingPermission,
  PermissionDecisionPayload,
} from "@/types/conversation/permission";

/** GroupChatPanel 发布的会话切片，Thread 面板据此派生展示数据。 */
export interface RoomThreadSource {
  conversation_id: string | null;
  message_groups: Map<string, Message[]>;
  pending_permission_groups: Map<string, PendingPermission[]>;
  pending_slot_groups: Map<string, RoomPendingAgentSlotState[]>;
  agent_name_map?: Record<string, string>;
  agent_avatar_map?: Record<string, string | null>;
  current_user_avatar?: string | null;
  // 发布前已用 callbacksRef 稳定，引用恒定。
  on_permission_response: (payload: PermissionDecisionPayload) => boolean;
  on_stop_message: (msgId: string) => void;
  on_open_workspace_file?: (path: string) => void;
}

interface RoomThreadLiveState {
  source: RoomThreadSource | null;
  set_source: (source: RoomThreadSource) => void;
  clear_source: () => void;
}

export const useRoomThreadLiveStore = create<RoomThreadLiveState>()((set) => ({
  source: null,
  set_source: (source) => set({ source }),
  clear_source: () => set({ source: null }),
}));
