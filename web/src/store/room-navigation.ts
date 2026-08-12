/**
 * Room 导航偏好 Store
 *
 * [INPUT]: Room 内显式会话选择、标签打开集合与有效会话路由
 * [OUTPUT]: 按 Room 持久化用户离开时的标签顺序和活动 Conversation
 * [POS]: store 模块的页面导航工作区状态，不参与服务端会话排序
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";

export interface RoomConversationTabsState {
  active_conversation_id: string;
  open_conversation_ids: string[];
}

interface PersistedRoomNavigationState {
  conversation_tabs_by_room?: Record<string, RoomConversationTabsState>;
  last_active_conversation_by_room?: Record<string, string>;
}

interface RoomNavigationState {
  conversation_tabs_by_room: Record<string, RoomConversationTabsState>;
  forget_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
  remember_last_active_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
  save_room_conversation_tabs: (
    roomId: string,
    openConversationIds: readonly string[],
    activeConversationId: string,
  ) => void;
}

export const useRoomNavigationStore = create<RoomNavigationState>()(
  persist(
    (set) => ({
      conversation_tabs_by_room: {},
      forget_conversation: (roomId, conversationId) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const normalizedConversationId = conversationId.trim();
        const currentTabs = state.conversation_tabs_by_room[normalizedRoomId];
        if (!normalizedRoomId || !normalizedConversationId || !currentTabs) {
          return state;
        }
        const openConversationIds = currentTabs.open_conversation_ids.filter(
          (id) => id !== normalizedConversationId,
        );
        if (openConversationIds.length === currentTabs.open_conversation_ids.length) {
          return state;
        }
        const nextTabs = openConversationIds.length > 0
          ? buildConversationTabsState(
              openConversationIds,
              currentTabs.active_conversation_id === normalizedConversationId
                ? openConversationIds[openConversationIds.length - 1]
                : currentTabs.active_conversation_id,
            )
          : null;
        const nextTabsByRoom = {...state.conversation_tabs_by_room};
        if (nextTabs) {
          nextTabsByRoom[normalizedRoomId] = nextTabs;
        } else {
          delete nextTabsByRoom[normalizedRoomId];
        }
        return {conversation_tabs_by_room: nextTabsByRoom};
      }),
      remember_last_active_conversation: (roomId, conversationId) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const normalizedConversationId = conversationId.trim();
        if (!normalizedRoomId || !normalizedConversationId) {
          return state;
        }
        const currentTabs = state.conversation_tabs_by_room[normalizedRoomId];
        const nextTabs = buildConversationTabsState(
          currentTabs?.open_conversation_ids ?? [],
          normalizedConversationId,
        );
        if (!nextTabs || areConversationTabsEqual(currentTabs, nextTabs)) {
          return state;
        }
        return {
          conversation_tabs_by_room: {
            ...state.conversation_tabs_by_room,
            [normalizedRoomId]: nextTabs,
          },
        };
      }),
      save_room_conversation_tabs: (
        roomId,
        openConversationIds,
        activeConversationId,
      ) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const nextTabs = buildConversationTabsState(
          openConversationIds,
          activeConversationId,
        );
        if (!normalizedRoomId || !nextTabs) {
          return state;
        }
        const currentTabs = state.conversation_tabs_by_room[normalizedRoomId];
        if (areConversationTabsEqual(currentTabs, nextTabs)) {
          return state;
        }
        return {
          conversation_tabs_by_room: {
            ...state.conversation_tabs_by_room,
            [normalizedRoomId]: nextTabs,
          },
        };
      }),
    }),
    {
      name: "nexus-room-navigation",
      partialize: (state) => ({
        conversation_tabs_by_room: state.conversation_tabs_by_room,
      }),
      storage: createBrowserJsonStorage(),
      version: 2,
      migrate: (persistedState: unknown): PersistedRoomNavigationState => {
        const state = (persistedState ?? {}) as PersistedRoomNavigationState;
        const conversationTabsByRoom = normalizePersistedConversationTabs(
          state.conversation_tabs_by_room,
        );
        for (const [roomId, conversationId] of Object.entries(
          state.last_active_conversation_by_room ?? {},
        )) {
          const normalizedRoomId = roomId.trim();
          const tabs = buildConversationTabsState([], conversationId);
          if (normalizedRoomId && tabs && !conversationTabsByRoom[normalizedRoomId]) {
            conversationTabsByRoom[normalizedRoomId] = tabs;
          }
        }
        return { conversation_tabs_by_room: conversationTabsByRoom };
      },
    },
  ),
);

function buildConversationTabsState(
  openConversationIds: readonly string[],
  activeConversationId: string,
): RoomConversationTabsState | null {
  const normalizedActiveId = activeConversationId.trim();
  if (!normalizedActiveId) {
    return null;
  }
  const normalizedOpenIds = [...new Set(
    openConversationIds.map((id) => id.trim()).filter(Boolean),
  )];
  if (!normalizedOpenIds.includes(normalizedActiveId)) {
    normalizedOpenIds.push(normalizedActiveId);
  }
  return {
    active_conversation_id: normalizedActiveId,
    open_conversation_ids: normalizedOpenIds,
  };
}

function normalizePersistedConversationTabs(
  tabsByRoom: Record<string, RoomConversationTabsState> | undefined,
): Record<string, RoomConversationTabsState> {
  const normalizedTabsByRoom: Record<string, RoomConversationTabsState> = {};
  for (const [roomId, tabs] of Object.entries(tabsByRoom ?? {})) {
    const normalizedRoomId = roomId.trim();
    const normalizedTabs = tabs && buildConversationTabsState(
      Array.isArray(tabs.open_conversation_ids) ? tabs.open_conversation_ids : [],
      typeof tabs.active_conversation_id === "string"
        ? tabs.active_conversation_id
        : "",
    );
    if (normalizedRoomId && normalizedTabs) {
      normalizedTabsByRoom[normalizedRoomId] = normalizedTabs;
    }
  }
  return normalizedTabsByRoom;
}

function areConversationTabsEqual(
  currentTabs: RoomConversationTabsState | undefined,
  nextTabs: RoomConversationTabsState,
): boolean {
  return currentTabs?.active_conversation_id === nextTabs.active_conversation_id
    && currentTabs.open_conversation_ids.length === nextTabs.open_conversation_ids.length
    && currentTabs.open_conversation_ids.every(
      (id, index) => id === nextTabs.open_conversation_ids[index],
    );
}
