/**
 * Room 导航偏好 Store
 *
 * [INPUT]: Room 内显式会话选择、标签打开集合、固定会话与有效会话路由
 * [OUTPUT]: 按 Room 持久化标签顺序和活动 Conversation，并保存全局固定会话偏好
 * [POS]: store 模块的页面导航工作区状态，不参与服务端会话排序
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

import { createBrowserJsonStorage } from "@/lib/storage/browser-storage";

export interface RoomConversationTabsState {
  active_conversation_id: string;
  open_conversation_ids: string[];
}

export interface PinnedConversationPreference {
  conversation_id: string;
  room_id: string;
  session_key: string;
  title: string;
}

interface PersistedRoomNavigationState {
  conversation_tabs_by_room?: Record<string, RoomConversationTabsState>;
  last_active_conversation_by_room?: Record<string, string>;
  pinned_conversations?: PinnedConversationPreference[];
}

interface RoomNavigationState {
  conversation_tabs_by_room: Record<string, RoomConversationTabsState>;
  pinned_conversations: PinnedConversationPreference[];
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
  toggle_pinned_conversation: (
    conversation: PinnedConversationPreference,
  ) => void;
  unpin_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
}

export const useRoomNavigationStore = create<RoomNavigationState>()(
  persist(
    (set) => ({
      conversation_tabs_by_room: {},
      pinned_conversations: [],
      forget_conversation: (roomId, conversationId) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const normalizedConversationId = conversationId.trim();
        const currentTabs = state.conversation_tabs_by_room[normalizedRoomId];
        if (!normalizedRoomId || !normalizedConversationId) {
          return state;
        }
        const nextPinnedConversations = state.pinned_conversations.filter(
          (item) => !matchesPinnedConversation(
            item,
            normalizedRoomId,
            normalizedConversationId,
          ),
        );
        const openConversationIds = currentTabs?.open_conversation_ids.filter(
          (id) => id !== normalizedConversationId,
        ) ?? [];
        const tabsChanged = Boolean(
          currentTabs
          && openConversationIds.length !== currentTabs.open_conversation_ids.length,
        );
        const pinChanged = nextPinnedConversations.length !== state.pinned_conversations.length;
        if (!tabsChanged && !pinChanged) {
          return state;
        }
        const nextTabs = currentTabs && openConversationIds.length > 0
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
        return {
          conversation_tabs_by_room: nextTabsByRoom,
          pinned_conversations: nextPinnedConversations,
        };
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
      toggle_pinned_conversation: (conversation) => set((state) => {
        const normalizedConversation = normalizePinnedConversation(conversation);
        if (!normalizedConversation) {
          return state;
        }
        const existingIndex = state.pinned_conversations.findIndex(
          (item) => matchesPinnedConversation(
            item,
            normalizedConversation.room_id,
            normalizedConversation.conversation_id,
          ),
        );
        if (existingIndex >= 0) {
          return {
            pinned_conversations: state.pinned_conversations.filter(
              (_, index) => index !== existingIndex,
            ),
          };
        }
        return {
          pinned_conversations: [
            ...state.pinned_conversations,
            normalizedConversation,
          ],
        };
      }),
      unpin_conversation: (roomId, conversationId) => set((state) => {
        const normalizedRoomId = roomId.trim();
        const normalizedConversationId = conversationId.trim();
        if (!normalizedRoomId || !normalizedConversationId) {
          return state;
        }
        const nextPinnedConversations = state.pinned_conversations.filter(
          (item) => !matchesPinnedConversation(
            item,
            normalizedRoomId,
            normalizedConversationId,
          ),
        );
        return nextPinnedConversations.length === state.pinned_conversations.length
          ? state
          : {pinned_conversations: nextPinnedConversations};
      }),
    }),
    {
      name: "nexus-room-navigation",
      partialize: (state) => ({
        conversation_tabs_by_room: state.conversation_tabs_by_room,
        pinned_conversations: state.pinned_conversations,
      }),
      storage: createBrowserJsonStorage(),
      // v3 曾短暂允许 active=null；v4 清洗标签，v5 加入全局固定会话。
      version: 5,
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
        const pinnedConversations = normalizePinnedConversations(
          state.pinned_conversations,
        );
        return pinnedConversations.length > 0
          ? {
              conversation_tabs_by_room: conversationTabsByRoom,
              pinned_conversations: pinnedConversations,
            }
          : {conversation_tabs_by_room: conversationTabsByRoom};
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

function normalizePinnedConversations(
  conversations: PinnedConversationPreference[] | undefined,
): PinnedConversationPreference[] {
  const normalizedConversations: PinnedConversationPreference[] = [];
  const seen = new Set<string>();
  for (const conversation of Array.isArray(conversations) ? conversations : []) {
    const normalizedConversation = normalizePinnedConversation(conversation);
    if (!normalizedConversation) {
      continue;
    }
    const key = getPinnedConversationKey(
      normalizedConversation.room_id,
      normalizedConversation.conversation_id,
    );
    if (seen.has(key)) {
      continue;
    }
    seen.add(key);
    normalizedConversations.push(normalizedConversation);
  }
  return normalizedConversations;
}

function normalizePinnedConversation(
  conversation: PinnedConversationPreference,
): PinnedConversationPreference | null {
  if (!conversation || typeof conversation !== "object") {
    return null;
  }
  const roomId = typeof conversation.room_id === "string"
    ? conversation.room_id.trim()
    : "";
  const conversationId = typeof conversation.conversation_id === "string"
    ? conversation.conversation_id.trim()
    : "";
  if (!roomId || !conversationId) {
    return null;
  }
  return {
    conversation_id: conversationId,
    room_id: roomId,
    session_key: typeof conversation.session_key === "string"
      ? conversation.session_key.trim()
      : "",
    title: typeof conversation.title === "string"
      ? conversation.title.trim()
      : "",
  };
}

function matchesPinnedConversation(
  conversation: PinnedConversationPreference,
  roomId: string,
  conversationId: string,
): boolean {
  return getPinnedConversationKey(
    conversation.room_id,
    conversation.conversation_id,
  ) === getPinnedConversationKey(roomId, conversationId);
}

function getPinnedConversationKey(roomId: string, conversationId: string): string {
  return `${roomId}\u0000${conversationId}`;
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
