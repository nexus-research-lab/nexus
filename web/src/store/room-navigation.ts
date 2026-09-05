/**
 * Room 导航偏好 Store
 *
 * [INPUT]: Room 导航命令、持久快照、应用注入的 owner 校验与 owner reset
 * [OUTPUT]: 合并当前 owner 最新快照的标签与固定偏好，跨页面恢复不覆盖其他导航操作
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

export type PinnedConversationPlacement = "after" | "before";

type PinnedConversationIdentity = Pick<
  PinnedConversationPreference,
  "conversation_id" | "room_id"
>;

interface PersistedRoomNavigationState {
  owner_scope?: string;
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
  reorder_pinned_conversation: (
    source: PinnedConversationIdentity,
    target: PinnedConversationIdentity,
    placement: PinnedConversationPlacement,
  ) => void;
  toggle_pinned_conversation: (
    conversation: PinnedConversationPreference,
  ) => void;
  unpin_conversation: (
    roomId: string,
    conversationId: string,
  ) => void;
}

const ROOM_NAVIGATION_STORAGE_KEY = "nexus-room-navigation";
let navigationOwnerGuard: (() => boolean) | null = null;
let navigationOwnerScope: string | null | undefined;
let suppressNavigationPersistence = false;
const navigationStorage = createBrowserJsonStorage();
type NavigationUpdate = (state: RoomNavigationState) => Partial<RoomNavigationState>;
type NavigationCommit = (update: NavigationUpdate) => void;
type RoomNavigationSnapshot = Pick<RoomNavigationState, "conversation_tabs_by_room" | "pinned_conversations">;

export const useRoomNavigationStore = create<RoomNavigationState>()(
  persist(
    withLatestRoomNavigation((set) => ({
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
      reorder_pinned_conversation: (source, target, placement) => set((state) => {
        const sourceKey = getNormalizedPinnedConversationKey(source);
        const targetKey = getNormalizedPinnedConversationKey(target);
        if (
          !sourceKey
          || !targetKey
          || sourceKey === targetKey
          || (placement !== "before" && placement !== "after")
        ) {
          return state;
        }
        const sourceIndex = state.pinned_conversations.findIndex(
          (conversation) => getPinnedConversationKey(
            conversation.room_id,
            conversation.conversation_id,
          ) === sourceKey,
        );
        const targetIndex = state.pinned_conversations.findIndex(
          (conversation) => getPinnedConversationKey(
            conversation.room_id,
            conversation.conversation_id,
          ) === targetKey,
        );
        if (sourceIndex < 0 || targetIndex < 0) {
          return state;
        }
        const nextPinnedConversations = [...state.pinned_conversations];
        const [movedConversation] = nextPinnedConversations.splice(sourceIndex, 1);
        const adjustedTargetIndex = nextPinnedConversations.findIndex(
          (conversation) => getPinnedConversationKey(
            conversation.room_id,
            conversation.conversation_id,
          ) === targetKey,
        );
        nextPinnedConversations.splice(
          adjustedTargetIndex + (placement === "after" ? 1 : 0),
          0,
          movedConversation,
        );
        if (nextPinnedConversations.every(
          (conversation, index) => conversation === state.pinned_conversations[index],
        )) {
          return state;
        }
        return {pinned_conversations: nextPinnedConversations};
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
    })),
    {
      name: ROOM_NAVIGATION_STORAGE_KEY,
      partialize: (state) => ({
        owner_scope: navigationOwnerScope ?? undefined,
        conversation_tabs_by_room: state.conversation_tabs_by_room,
        pinned_conversations: state.pinned_conversations,
      }),
      storage: navigationStorage && {
        ...navigationStorage,
        setItem: (name, value) => {
          if (!suppressNavigationPersistence) return navigationStorage.setItem(name, value);
        },
      },
      // v3 曾短暂允许 active=null；v4 清洗标签，v5 加入全局固定会话。
      version: 5,
      merge: (persistedState, currentState) => {
        if ((navigationOwnerGuard && !navigationOwnerGuard())
          || (persistedState as PersistedRoomNavigationState | undefined)?.owner_scope !== navigationOwnerScope) {
          return currentState;
        }
        const saved = normalizeRoomNavigation(persistedState);
        return areNavigationSnapshotsEqual(currentState, saved) ? currentState : { ...currentState, ...saved };
      },
      migrate: normalizeRoomNavigation,
    },
  ),
);

function withLatestRoomNavigation(createState: (set: NavigationCommit) => RoomNavigationState) {
  return (commit: NavigationCommit) => createState((update) => {
    // A dormant page can still hold an old pin list. Apply its command to
    // the latest same-owner snapshot before persist writes the whole store.
    if (navigationOwnerGuard && !navigationOwnerGuard()) return;
    const saved = navigationOwnerGuard ? readLatestRoomNavigation() : null;
    if (navigationOwnerGuard && !navigationOwnerGuard()) return;
    commit((current) => {
      const latest = saved && !areNavigationSnapshotsEqual(current, saved) ? { ...current, ...saved } : current;
      const changed = update(latest);
      return changed === latest ? latest : { ...latest, ...changed };
    });
  });
}

/** App owns identity proof; this store only consumes its current-owner guard. */
export function setRoomNavigationOwnerScope(ownerScope: string | null, guard: () => boolean): void {
  const firstBinding = navigationOwnerScope !== ownerScope;
  navigationOwnerScope = ownerScope;
  navigationOwnerGuard = guard;
  if (ownerScope === null) {
    // Only an authoritative signed-out binding clears the shared snapshot.
    useRoomNavigationStore.setState({ conversation_tabs_by_room: {}, pinned_conversations: [] });
  } else if (firstBinding && guard()) {
    // Tagged snapshots stay hidden until auth binds their owner. Untagged v5
    // data is adopted only after App's existing legacy-owner reset has run.
    const saved = readLatestRoomNavigation();
    useRoomNavigationStore.setState(saved ?? {});
  }
}

/** Storage events are invalidation signals; read current storage, never event payloads. */
export function synchronizeRoomNavigationStorage(event: StorageEvent): void {
  if (event.key === ROOM_NAVIGATION_STORAGE_KEY && navigationOwnerGuard?.()) {
    void useRoomNavigationStore.persist.rehydrate();
  }
}

function readLatestRoomNavigation(): RoomNavigationSnapshot | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(ROOM_NAVIGATION_STORAGE_KEY);
    if (!raw) return null;
    const saved = JSON.parse(raw).state as PersistedRoomNavigationState;
    return saved?.owner_scope === navigationOwnerScope ? normalizeRoomNavigation(saved) : null;
  } catch {
    // Unavailable storage cannot provide another page's state; keep local state.
    return null;
  }
}

function normalizeRoomNavigation(persistedState: unknown): RoomNavigationSnapshot {
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
  return {
    conversation_tabs_by_room: conversationTabsByRoom,
    pinned_conversations: pinnedConversations,
  };
}

function areNavigationSnapshotsEqual(current: RoomNavigationSnapshot, next: RoomNavigationSnapshot): boolean {
  const rooms = Object.keys(current.conversation_tabs_by_room);
  return rooms.length === Object.keys(next.conversation_tabs_by_room).length
    && rooms.every((id) => {
      const nextTabs = next.conversation_tabs_by_room[id];
      return nextTabs && areConversationTabsEqual(current.conversation_tabs_by_room[id], nextTabs);
    })
    && current.pinned_conversations.length === next.pinned_conversations.length
    && current.pinned_conversations.every((pin, index) => {
      const other = next.pinned_conversations[index];
      return pin.room_id === other.room_id && pin.conversation_id === other.conversation_id
        && pin.session_key === other.session_key && pin.title === other.title;
    });
}

/** 清空本页并阻止迟到命令；持久快照留给 App 下一次权威身份绑定处理。 */
export function resetRoomNavigationOwnerScope(): void {
  navigationOwnerGuard = () => false;
  navigationOwnerScope = undefined;
  suppressNavigationPersistence = true;
  try {
    useRoomNavigationStore.setState({
      conversation_tabs_by_room: {},
      pinned_conversations: [],
    });
  } finally {
    suppressNavigationPersistence = false;
  }
}

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

function getNormalizedPinnedConversationKey(
  conversation: PinnedConversationIdentity,
): string | null {
  const roomId = conversation.room_id.trim();
  const conversationId = conversation.conversation_id.trim();
  return roomId && conversationId
    ? getPinnedConversationKey(roomId, conversationId)
    : null;
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
