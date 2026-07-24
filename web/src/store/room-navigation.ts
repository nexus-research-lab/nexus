/**
 * Room 导航偏好 Store
 *
 * [INPUT]: Room 内显式会话选择与有效会话路由
 * [OUTPUT]: 按 Room 持久化用户最后激活的 Conversation
 * [POS]: store 模块的页面恢复状态，不参与服务端会话排序
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";

interface RoomNavigationState {
  last_active_conversation_by_room: Record<string, string>;
  remember_last_active_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
}

export const useRoomNavigationStore = create<RoomNavigationState>()(
  persist(
    (set) => ({
      last_active_conversation_by_room: {},
      remember_last_active_conversation: (roomId, conversationId) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const normalizedConversationId = conversationId.trim();
        if (!normalizedRoomId || !normalizedConversationId) {
          return state;
        }
        if (
          state.last_active_conversation_by_room[normalizedRoomId]
          === normalizedConversationId
        ) {
          return state;
        }
        return {
          last_active_conversation_by_room: {
            ...state.last_active_conversation_by_room,
            [normalizedRoomId]: normalizedConversationId,
          },
        };
      }),
    }),
    {
      name: "nexus-room-navigation",
      partialize: (state) => ({
        last_active_conversation_by_room: state.last_active_conversation_by_room,
      }),
      storage: createBrowserJsonStorage(),
      version: 1,
    },
  ),
);
