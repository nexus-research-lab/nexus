/**
 * 侧边栏状态 Store
 *
 * 当前侧栏只保留宽面板本体，
 * 这里集中管理列表高亮、分区折叠和面板宽度。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

/** 宽面板宽度约束 */
const WIDE_PANEL_MIN_WIDTH = 264;
const WIDE_PANEL_MAX_WIDTH = 400;
const WIDE_PANEL_DEFAULT_WIDTH = 264;
type WidePanelCollapseSource = "manual" | "right_panel_auto";
export const SIDEBAR_SYSTEM_ITEM_IDS = {
  nexus: "system:nexus",
} as const;
export const SIDEBAR_CAPABILITY_ITEM_IDS = {
  skills: "capability:skills",
  loops: "capability:loops",
  connectors: "capability:connectors",
  scheduledTasks: "capability:scheduled-tasks",
  channels: "capability:channels",
  pairings: "capability:pairings",
} as const;

/** 根据当前路由派生侧栏高亮条目，保证整套导航只走一个状态源。 */
export function deriveSidebarItemIdFromPath(pathname: string): string | null {
  if (pathname.startsWith("/capability/skills")) return SIDEBAR_CAPABILITY_ITEM_IDS.skills;
  if (pathname.startsWith("/capability/loops")) return SIDEBAR_CAPABILITY_ITEM_IDS.loops;
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

interface SidebarState {
  /** 宽面板中当前高亮的条目 ID（Room/DM/Agent/Skill） */
  active_panel_item_id: string | null;
  /** 主智能体 DM 的真实 roomId，用于 header 入口和真实 room 路由共用同一激活语义。 */
  nexus_room_id: string | null;
  /** 宽面板宽度（px），支持拖拽调整 */
  wide_panel_width: number;
  /** 宽面板是否处于收起状态。 */
  wide_panel_collapsed: boolean;
  /** 记录收起来源，避免右侧面板自动收起覆盖用户手动选择。 */
  wide_panel_collapse_source: WidePanelCollapseSource | null;
  /** 聊天入口未读消息提示数量。 */
  chat_badge_count: number;
  /** 聊天会话维度的未读完成消息数。 */
  chat_unread_counts: Record<string, number>;
  /** 未读目标元数据，用于列表按 Room 聚合并跳转到真实未读会话。 */
  chat_unread_targets: Record<string, ChatNotificationTargetState>;
  /** 未读目标最后更新时间，用于点击列表时优先进入最新未读会话。 */
  chat_unread_timestamps: Record<string, number>;
  /** 已计入通知的消息 ID，避免 WebSocket 重放或多订阅重复提示。 */
  notified_chat_message_ids: string[];
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
}

interface SidebarActions {
  set_active_panel_item: (id: string | null) => void;
  set_nexus_room_id: (roomId: string | null) => void;
  set_chat_badge_count: (count: number) => void;
  record_chat_notification: (target: ChatNotificationTargetState, messageId: string) => boolean;
  clear_chat_notifications_for_target: (targetKey: string | null | undefined) => void;
  clear_chat_notifications_for_room: (roomId: string | null | undefined) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  set_wide_panel_collapsed: (collapsed: boolean) => void;
  toggle_wide_panel_collapsed: () => void;
  collapse_wide_panel_for_right_panel: () => void;
  expand_wide_panel_after_right_panel: () => void;
  toggle_section: (sectionId: string) => void;
}

const MAX_NOTIFIED_CHAT_MESSAGE_IDS = 300;

function countChatUnreadTotal(counts: Record<string, number>): number {
  return Object.values(counts).reduce((total, count) => total + Math.max(0, count), 0);
}

function clearChatUnreadKeys(
  state: SidebarState,
  keys: string[],
): Pick<SidebarState, "chat_badge_count" | "chat_unread_counts" | "chat_unread_targets" | "chat_unread_timestamps"> {
  const uniqueKeys = Array.from(new Set(keys.filter(Boolean)));
  if (uniqueKeys.length === 0) {
    return {
      chat_badge_count: state.chat_badge_count,
      chat_unread_counts: state.chat_unread_counts,
      chat_unread_targets: state.chat_unread_targets,
      chat_unread_timestamps: state.chat_unread_timestamps,
    };
  }

  const nextCounts = { ...state.chat_unread_counts };
  const nextTargets = { ...state.chat_unread_targets };
  const nextTimestamps = { ...state.chat_unread_timestamps };
  for (const key of uniqueKeys) {
    delete nextCounts[key];
    delete nextTargets[key];
    delete nextTimestamps[key];
  }
  return {
    chat_badge_count: countChatUnreadTotal(nextCounts),
    chat_unread_counts: nextCounts,
    chat_unread_targets: nextTargets,
    chat_unread_timestamps: nextTimestamps,
  };
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_panel_item_id: null,
      nexus_room_id: null,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      wide_panel_collapsed: false,
      wide_panel_collapse_source: null,
      chat_badge_count: 0,
      chat_unread_counts: {},
      chat_unread_targets: {},
      chat_unread_timestamps: {},
      notified_chat_message_ids: [],
      collapsed_sections: {},

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),
      set_nexus_room_id: (roomId) => set({ nexus_room_id: roomId }),
      set_chat_badge_count: (count) => set({ chat_badge_count: Math.max(0, Math.floor(count)) }),
      record_chat_notification: (target, messageId) => {
        let didRecord = false;
        set((state) => {
          const normalizedTargetKey = target.key.trim();
          const normalizedMessageId = messageId.trim();
          if (!normalizedTargetKey || !normalizedMessageId) {
            return state;
          }
          if (state.notified_chat_message_ids.includes(normalizedMessageId)) {
            return state;
          }

          didRecord = true;
          const nextCounts = {
            ...state.chat_unread_counts,
            [normalizedTargetKey]: (state.chat_unread_counts[normalizedTargetKey] ?? 0) + 1,
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
          const nextMessageIds = [
            normalizedMessageId,
            ...state.notified_chat_message_ids,
          ].slice(0, MAX_NOTIFIED_CHAT_MESSAGE_IDS);
          return {
            chat_badge_count: countChatUnreadTotal(nextCounts),
            chat_unread_counts: nextCounts,
            chat_unread_targets: nextTargets,
            chat_unread_timestamps: nextTimestamps,
            notified_chat_message_ids: nextMessageIds,
          };
        });
        return didRecord;
      },
      clear_chat_notifications_for_target: (targetKey) => set((state) => {
        const normalizedTargetKey = targetKey?.trim();
        if (!normalizedTargetKey || !state.chat_unread_counts[normalizedTargetKey]) {
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
        if (keys.length === 0) {
          return state;
        }
        return clearChatUnreadKeys(state, keys);
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
