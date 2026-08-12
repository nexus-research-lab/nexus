/**
 * Room 导航偏好 Store
 *
 * [INPUT]: Room 内显式会话选择、标签打开集合与有效会话路由
 * [OUTPUT]: 按 Room 持久化标签顺序、活动 Conversation 与明确关闭全部的空快照
 * [POS]: store 模块的页面导航工作区状态，不参与服务端会话排序
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";

export interface RoomConversationTabsState {
  active_conversation_id: string | null;
  open_conversation_ids: string[];
}

interface PersistedRoomNavigationState {
  conversation_tabs_by_room?: Record<string, RoomConversationTabsState>;
  last_active_conversation_by_room?: Record<string, string>;
}

interface RoomNavigationState {
  conversation_tabs_by_room: Record<string, RoomConversationTabsState>;
  remember_last_active_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
  save_room_conversation_tabs: (
    roomId: string,
    openConversationIds: readonly string[],
    activeConversationId: string | null,
  ) => void;
}

export const useRoomNavigationStore = create<RoomNavigationState>()(
  persist(
    (set) => ({
      conversation_tabs_by_room: {},
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
        if (!normalizedRoomId) {
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
      version: 3,
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
          if (normalizedRoomId && !conversationTabsByRoom[normalizedRoomId]) {
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
  activeConversationId: string | null,
): RoomConversationTabsState {
  const normalizedOpenIds = [...new Set(
    openConversationIds.map((id) => id.trim()).filter(Boolean),
  )];
  const normalizedActiveId = activeConversationId?.trim() || null;
  if (!normalizedActiveId) {
    return {
      active_conversation_id: null,
      open_conversation_ids: [],
    };
  }
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
    let activeConversationId: string | null | undefined;
    if (typeof tabs?.active_conversation_id === "string") {
      activeConversationId = tabs.active_conversation_id;
    } else if (tabs?.active_conversation_id === null) {
      activeConversationId = null;
    }
    if (!normalizedRoomId || activeConversationId === undefined) {
      continue;
    }
    const normalizedTabs = buildConversationTabsState(
      Array.isArray(tabs.open_conversation_ids) ? tabs.open_conversation_ids : [],
      activeConversationId,
    );
    normalizedTabsByRoom[normalizedRoomId] = normalizedTabs;
  }
  return normalizedTabsByRoom;
}

function areConversationTabsEqual(
  currentTabs: RoomConversationTabsState | undefined,
  nextTabs: RoomConversationTabsState,
): boolean {
  return currentTabs !== undefined
    && currentTabs.active_conversation_id === nextTabs.active_conversation_id
    && currentTabs.open_conversation_ids.length === nextTabs.open_conversation_ids.length
    && currentTabs.open_conversation_ids.every(
      (id, index) => id === nextTabs.open_conversation_ids[index],
    );
}
