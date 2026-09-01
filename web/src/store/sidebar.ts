/**
 * INPUT: 侧栏布局命令、聊天完成通知及其目标路由顺序信息。
 * OUTPUT: 导航高亮、聊天入口红点、会话未读导航数据、分区折叠、面板宽度与 owner reset。
 * POS: 侧栏运行态 Store；owner 切换清除路由/未读，布局偏好由 persist 单独保存。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

/** 宽面板宽度约束 */
const WIDE_PANEL_MIN_WIDTH = 264;
const WIDE_PANEL_MAX_WIDTH = 400;
const WIDE_PANEL_DEFAULT_WIDTH = 264;
type WidePanelCollapseSource = "manual" | "right_panel_auto";
export const SIDEBAR_CAPABILITY_ITEM_IDS = {
  skills: "capability:skills",
  loops: "capability:loops",
  workGraphDistillations: "capability:workgraphs",
  connectors: "capability:connectors",
  scheduledTasks: "capability:scheduled-tasks",
  channels: "capability:channels",
  pairings: "capability:pairings",
} as const;

/** 根据当前路由派生侧栏高亮条目，保证整套导航只走一个状态源。 */
export function deriveSidebarItemIdFromPath(pathname: string): string | null {
  if (pathname.startsWith("/capability/skills")) return SIDEBAR_CAPABILITY_ITEM_IDS.skills;
  if (pathname.startsWith("/capability/loops")) return SIDEBAR_CAPABILITY_ITEM_IDS.loops;
  if (pathname.startsWith("/capability/workgraphs")) return SIDEBAR_CAPABILITY_ITEM_IDS.workGraphDistillations;
  if (pathname.startsWith("/capability/connectors")) return SIDEBAR_CAPABILITY_ITEM_IDS.connectors;
  if (pathname.startsWith("/capability/scheduled-tasks")) return SIDEBAR_CAPABILITY_ITEM_IDS.scheduledTasks;
  if (pathname.startsWith("/capability/channels")) return SIDEBAR_CAPABILITY_ITEM_IDS.channels;
  if (pathname.startsWith("/capability/pairings")) return SIDEBAR_CAPABILITY_ITEM_IDS.pairings;

  if (pathname.startsWith("/rooms/")) {
    const roomId = pathname.split("/")[2];
    return roomId ? decodeURIComponent(roomId) : null;
  }

  return null;
}

/** 将宽度限制在合法范围内 */
function clampPanelWidth(width: number): number {
  return Math.round(Math.min(WIDE_PANEL_MAX_WIDTH, Math.max(WIDE_PANEL_MIN_WIDTH, width)));
}

export interface ChatNotificationTargetState {
  key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  session_key?: string | null;
}

export interface ChatUnreadMessageAnchor {
  agent_id?: string | null;
  agent_round_id?: string | null;
  message_id: string;
  room_seq?: number | null;
  round_id?: string | null;
  timestamp: number;
}

export interface ChatUnreadAnchorState extends ChatNotificationTargetState {
  messages: ChatUnreadMessageAnchor[];
}

export interface RecordChatNotificationOptions {
  preserve_anchor: boolean;
}

interface SidebarState {
  /** 宽面板中当前高亮的条目 ID（Room/DM/Skill） */
  active_panel_item_id: string | null;
  /** 宽面板宽度（px），支持拖拽调整 */
  wide_panel_width: number;
  /** 宽面板是否处于收起状态。 */
  wide_panel_collapsed: boolean;
  /** 记录收起来源，避免右侧面板自动收起覆盖用户手动选择。 */
  wide_panel_collapse_source: WidePanelCollapseSource | null;
  /** 聊天入口未读消息提示数量。 */
  chat_badge_count: number;
  /** 自上次进入聊天页后新增的会话维度计数；不影响会话内未读锚点。 */
  chat_tab_unseen_counts: Record<string, number>;
  /** 聊天会话维度的未读完成消息数。 */
  chat_unread_counts: Record<string, number>;
  /** 未读目标元数据，用于列表按 Room 聚合并跳转到真实未读会话。 */
  chat_unread_targets: Record<string, ChatNotificationTargetState>;
  /** 未读目标最后更新时间，用于点击列表时优先进入最新未读会话。 */
  chat_unread_timestamps: Record<string, number>;
  /** 每个精确目标的未读完成消息顺序，仅用于侧栏选择最早未读 Conversation。 */
  chat_unread_anchors: Record<string, ChatUnreadAnchorState>;
  /** 已计入通知的消息 ID，避免 WebSocket 重放或多订阅重复提示。 */
  notified_chat_message_ids: string[];
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
}

interface SidebarActions {
  set_active_panel_item: (id: string | null) => void;
  acknowledge_chat_tab: () => void;
  record_chat_notification: (
    target: ChatNotificationTargetState,
    message: ChatUnreadMessageAnchor,
    options?: RecordChatNotificationOptions,
  ) => boolean;
  clear_chat_notifications_for_target: (targetKey: string | null | undefined) => void;
  clear_chat_notifications_for_room: (roomId: string | null | undefined) => void;
  discard_chat_state_for_room: (roomId: string | null | undefined) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  set_wide_panel_collapsed: (collapsed: boolean) => void;
  toggle_wide_panel_collapsed: () => void;
  collapse_wide_panel_for_right_panel: () => void;
  expand_wide_panel_after_right_panel: () => void;
  toggle_section: (sectionId: string) => void;
}

const MAX_NOTIFIED_CHAT_MESSAGE_IDS = 300;
const DEFAULT_RECORD_CHAT_NOTIFICATION_OPTIONS: RecordChatNotificationOptions = {
  preserve_anchor: true,
};

function countChatUnreadTotal(counts: Record<string, number>): number {
  return Object.values(counts).reduce((total, count) => total + Math.max(0, count), 0);
}

function clearChatUnreadKeys(
  state: SidebarState,
  keys: string[],
): Pick<
  SidebarState,
  | "chat_badge_count"
  | "chat_tab_unseen_counts"
  | "chat_unread_anchors"
  | "chat_unread_counts"
  | "chat_unread_targets"
  | "chat_unread_timestamps"
> {
  const uniqueKeys = Array.from(new Set(keys.filter(Boolean)));
  if (uniqueKeys.length === 0) {
    return {
      chat_badge_count: state.chat_badge_count,
      chat_tab_unseen_counts: state.chat_tab_unseen_counts,
      chat_unread_anchors: state.chat_unread_anchors,
      chat_unread_counts: state.chat_unread_counts,
      chat_unread_targets: state.chat_unread_targets,
      chat_unread_timestamps: state.chat_unread_timestamps,
    };
  }

  const nextCounts = { ...state.chat_unread_counts };
  const nextTabUnseenCounts = { ...state.chat_tab_unseen_counts };
  const nextAnchors = { ...state.chat_unread_anchors };
  const nextTargets = { ...state.chat_unread_targets };
  const nextTimestamps = { ...state.chat_unread_timestamps };
  for (const key of uniqueKeys) {
    delete nextCounts[key];
    delete nextTabUnseenCounts[key];
    delete nextAnchors[key];
    delete nextTargets[key];
    delete nextTimestamps[key];
  }
  return {
    chat_badge_count: countChatUnreadTotal(nextTabUnseenCounts),
    chat_tab_unseen_counts: nextTabUnseenCounts,
    chat_unread_anchors: nextAnchors,
    chat_unread_counts: nextCounts,
    chat_unread_targets: nextTargets,
    chat_unread_timestamps: nextTimestamps,
  };
}

function compareChatUnreadMessages(
  left: ChatUnreadMessageAnchor,
  right: ChatUnreadMessageAnchor,
): number {
  const leftSequence = Number.isFinite(left.room_seq)
    ? Number(left.room_seq)
    : Number.POSITIVE_INFINITY;
  const rightSequence = Number.isFinite(right.room_seq)
    ? Number(right.room_seq)
    : Number.POSITIVE_INFINITY;
  return leftSequence - rightSequence
    || left.timestamp - right.timestamp
    || left.message_id.localeCompare(right.message_id);
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_panel_item_id: null,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      wide_panel_collapsed: false,
      wide_panel_collapse_source: null,
      chat_badge_count: 0,
      chat_tab_unseen_counts: {},
      chat_unread_counts: {},
      chat_unread_targets: {},
      chat_unread_timestamps: {},
      chat_unread_anchors: {},
      notified_chat_message_ids: [],
      collapsed_sections: {},

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),
      acknowledge_chat_tab: () => set({
        chat_badge_count: 0,
        chat_tab_unseen_counts: {},
      }),
      record_chat_notification: (
        target,
        message,
        options = DEFAULT_RECORD_CHAT_NOTIFICATION_OPTIONS,
      ) => {
        let didRecord = false;
        set((state) => {
          const normalizedTargetKey = target.key.trim();
          const normalizedMessageId = message.message_id.trim();
          if (!normalizedTargetKey || !normalizedMessageId) {
            return state;
          }
          const notificationIdentity =
            `${normalizedTargetKey}\u001f${normalizedMessageId}`;
          const currentAnchor = state.chat_unread_anchors[normalizedTargetKey];
          if (
            state.notified_chat_message_ids.includes(notificationIdentity)
            || currentAnchor?.messages.some(
              (candidate) => candidate.message_id === normalizedMessageId,
            )
          ) {
            return state;
          }

          didRecord = true;
          const nextCounts = {
            ...state.chat_unread_counts,
            [normalizedTargetKey]:
              (state.chat_unread_counts[normalizedTargetKey] ?? 0) + 1,
          };
          const nextTabUnseenCounts = {
            ...state.chat_tab_unseen_counts,
            [normalizedTargetKey]:
              (state.chat_tab_unseen_counts[normalizedTargetKey] ?? 0) + 1,
          };
          const nextTargets = {
            ...state.chat_unread_targets,
            [normalizedTargetKey]: {
              ...target,
              key: normalizedTargetKey,
            },
          };
          const nextTimestamps = {
            ...state.chat_unread_timestamps,
            [normalizedTargetKey]: Date.now(),
          };
          const nextAnchors = options.preserve_anchor
            ? {
                ...state.chat_unread_anchors,
                [normalizedTargetKey]: {
                  ...(currentAnchor ?? {
                    ...target,
                    key: normalizedTargetKey,
                    messages: [],
                  }),
                  messages: [
                    ...(currentAnchor?.messages ?? []),
                    {
                      ...message,
                      message_id: normalizedMessageId,
                    },
                  ].sort(compareChatUnreadMessages),
                },
              }
            : state.chat_unread_anchors;
          const nextMessageIds = [
            notificationIdentity,
            ...state.notified_chat_message_ids,
          ].slice(0, MAX_NOTIFIED_CHAT_MESSAGE_IDS);
          return {
            chat_badge_count: countChatUnreadTotal(nextTabUnseenCounts),
            chat_tab_unseen_counts: nextTabUnseenCounts,
            chat_unread_counts: nextCounts,
            chat_unread_targets: nextTargets,
            chat_unread_timestamps: nextTimestamps,
            chat_unread_anchors: nextAnchors,
            notified_chat_message_ids: nextMessageIds,
          };
        });
        return didRecord;
      },
      clear_chat_notifications_for_target: (targetKey) => set((state) => {
        const normalizedTargetKey = targetKey?.trim();
        if (
          !normalizedTargetKey
          || (
            !state.chat_unread_counts[normalizedTargetKey]
            && !state.chat_unread_anchors[normalizedTargetKey]
          )
        ) {
          return state;
        }
        return clearChatUnreadKeys(state, [normalizedTargetKey]);
      }),
      clear_chat_notifications_for_room: (roomId) => set((state) => {
        const normalizedRoomId = roomId?.trim();
        if (!normalizedRoomId) {
          return state;
        }
        const roomKey = `room:${normalizedRoomId}`;
        const roomConversationKeyPrefix = `${roomKey}:conversation:`;
        const keys = Object.entries(state.chat_unread_targets)
          .filter(([, target]) => target.room_id === normalizedRoomId)
          .map(([key]) => key);
        for (const key of Object.keys(state.chat_unread_counts)) {
          if (key === roomKey || key.startsWith(roomConversationKeyPrefix)) {
            keys.push(key);
          }
        }
        for (const [key, anchor] of Object.entries(state.chat_unread_anchors)) {
          if (
            anchor.room_id === normalizedRoomId
            || key === roomKey
            || key.startsWith(roomConversationKeyPrefix)
          ) {
            keys.push(key);
          }
        }
        if (keys.length === 0) {
          return state;
        }
        return clearChatUnreadKeys(state, keys);
      }),
      discard_chat_state_for_room: (roomId) => set((state) => {
        const normalizedRoomId = roomId?.trim();
        if (!normalizedRoomId) {
          return state;
        }
        const roomKey = `room:${normalizedRoomId}`;
        const roomConversationKeyPrefix = `${roomKey}:conversation:`;
        const keys = new Set<string>();
        for (const key of Object.keys(state.chat_unread_counts)) {
          if (key === roomKey || key.startsWith(roomConversationKeyPrefix)) {
            keys.add(key);
          }
        }
        for (const [key, target] of Object.entries({
          ...state.chat_unread_targets,
          ...state.chat_unread_anchors,
        })) {
          if (
            target.room_id === normalizedRoomId
            || key === roomKey
            || key.startsWith(roomConversationKeyPrefix)
          ) {
            keys.add(key);
          }
        }
        const cleared = clearChatUnreadKeys(state, Array.from(keys));
        const notificationPrefixes = Array.from(keys, (key) => `${key}\u001f`);
        return {
          ...cleared,
          notified_chat_message_ids: state.notified_chat_message_ids.filter(
            (identity) => !notificationPrefixes.some(
              (prefix) => identity.startsWith(prefix),
            ),
          ),
        };
      }),

      set_wide_panel_width: (width) =>
        set({ wide_panel_width: clampPanelWidth(width) }),
      set_wide_panel_collapsed: (collapsed) =>
        set({
          wide_panel_collapsed: collapsed,
          wide_panel_collapse_source: collapsed ? "manual" : null,
        }),
      toggle_wide_panel_collapsed: () =>
        set((state) => ({
          wide_panel_collapsed: !state.wide_panel_collapsed,
          wide_panel_collapse_source: !state.wide_panel_collapsed ? "manual" : null,
        })),
      collapse_wide_panel_for_right_panel: () =>
        set((state) => {
          if (state.wide_panel_collapsed) {
            return state;
          }
          return {
            wide_panel_collapsed: true,
            wide_panel_collapse_source: "right_panel_auto",
          };
        }),
      expand_wide_panel_after_right_panel: () =>
        set((state) => {
          if (state.wide_panel_collapse_source !== "right_panel_auto") {
            return state;
          }
          return {
            wide_panel_collapsed: false,
            wide_panel_collapse_source: null,
          };
        }),

      toggle_section: (sectionId) =>
        set((state) => ({
          collapsed_sections: {
            ...state.collapsed_sections,
            [sectionId]: !state.collapsed_sections[sectionId],
          },
        })),
    }),
    {
      name: "nexus-sidebar",
      // 只持久化布局相关状态，条目高亮保持运行时态
      partialize: (state) => ({
        wide_panel_width: state.wide_panel_width,
        wide_panel_collapsed: state.wide_panel_collapse_source === "manual"
          ? state.wide_panel_collapsed
          : false,
        wide_panel_collapse_source: state.wide_panel_collapse_source === "manual"
          ? state.wide_panel_collapse_source
          : null,
        collapsed_sections: state.collapsed_sections,
      }),
    },
  ),
);

/** Auth owner 变化时只清除身份相关运行态，保留纯布局偏好。 */
export function resetSidebarOwnerScope(): void {
  useSidebarStore.setState({
    active_panel_item_id: null,
    chat_badge_count: 0,
    chat_tab_unseen_counts: {},
    chat_unread_counts: {},
    chat_unread_targets: {},
    chat_unread_timestamps: {},
    chat_unread_anchors: {},
    notified_chat_message_ids: [],
  });
}
