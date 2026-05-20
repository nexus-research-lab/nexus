/**
 * 侧边栏状态 Store
 *
 * 当前侧栏只保留宽面板本体，
 * 这里集中管理列表高亮、分区折叠和面板宽度。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

/** 宽面板宽度约束 */
export const WIDE_PANEL_MIN_WIDTH = 264;
export const WIDE_PANEL_MAX_WIDTH = 400;
export const WIDE_PANEL_DEFAULT_WIDTH = 264;
export const SIDEBAR_SYSTEM_ITEM_IDS = {
  nexus: "system:nexus",
} as const;
export const SIDEBAR_CAPABILITY_ITEM_IDS = {
  skills: "capability:skills",
  connectors: "capability:connectors",
  scheduled_tasks: "capability:scheduled-tasks",
  channels: "capability:channels",
  pairings: "capability:pairings",
} as const;

/** 根据当前路由派生侧栏高亮条目，保证整套导航只走一个状态源。 */
export function derive_sidebar_item_id_from_path(pathname: string): string | null {
  if (pathname.startsWith("/capability/skills")) return SIDEBAR_CAPABILITY_ITEM_IDS.skills;
  if (pathname.startsWith("/capability/connectors")) return SIDEBAR_CAPABILITY_ITEM_IDS.connectors;
  if (pathname.startsWith("/capability/scheduled-tasks")) return SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks;
  if (pathname.startsWith("/capability/channels")) return SIDEBAR_CAPABILITY_ITEM_IDS.channels;
  if (pathname.startsWith("/capability/pairings")) return SIDEBAR_CAPABILITY_ITEM_IDS.pairings;

  if (pathname.startsWith("/rooms/")) {
    const room_id = pathname.split("/")[2];
    return room_id ? decodeURIComponent(room_id) : null;
  }

  return null;
}

/** 将宽度限制在合法范围内 */
function clamp_panel_width(width: number): number {
  return Math.round(Math.min(WIDE_PANEL_MAX_WIDTH, Math.max(WIDE_PANEL_MIN_WIDTH, width)));
}

interface SidebarState {
  /** 宽面板中当前高亮的条目 ID（Room/DM/Agent/Skill） */
  active_panel_item_id: string | null;
  /** 主智能体 DM 的真实 room_id，用于 header 入口和真实 room 路由共用同一激活语义。 */
  nexus_room_id: string | null;
  /** 宽面板宽度（px），支持拖拽调整 */
  wide_panel_width: number;
  /** 聊天入口未读消息提示数量。 */
  chat_badge_count: number;
  /** 聊天会话维度的未读完成消息数。 */
  chat_unread_counts: Record<string, number>;
  /** 已计入通知的消息 ID，避免 WebSocket 重放或多订阅重复提示。 */
  notified_chat_message_ids: string[];
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
}

interface SidebarActions {
  set_active_panel_item: (id: string | null) => void;
  set_nexus_room_id: (room_id: string | null) => void;
  set_chat_badge_count: (count: number) => void;
  record_chat_notification: (target_key: string, message_id: string) => boolean;
  clear_chat_notifications_for_target: (target_key: string | null | undefined) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  toggle_section: (section_id: string) => void;
}

const MAX_NOTIFIED_CHAT_MESSAGE_IDS = 300;

function count_chat_unread_total(counts: Record<string, number>): number {
  return Object.values(counts).reduce((total, count) => total + Math.max(0, count), 0);
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_panel_item_id: null,
      nexus_room_id: null,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      chat_badge_count: 0,
      chat_unread_counts: {},
      notified_chat_message_ids: [],
      collapsed_sections: {},

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),
      set_nexus_room_id: (room_id) => set({ nexus_room_id: room_id }),
      set_chat_badge_count: (count) => set({ chat_badge_count: Math.max(0, Math.floor(count)) }),
      record_chat_notification: (target_key, message_id) => {
        let did_record = false;
        set((state) => {
          const normalized_target_key = target_key.trim();
          const normalized_message_id = message_id.trim();
          if (!normalized_target_key || !normalized_message_id) {
            return state;
          }
          if (state.notified_chat_message_ids.includes(normalized_message_id)) {
            return state;
          }

          did_record = true;
          const next_counts = {
            ...state.chat_unread_counts,
            [normalized_target_key]: (state.chat_unread_counts[normalized_target_key] ?? 0) + 1,
          };
          const next_message_ids = [
            normalized_message_id,
            ...state.notified_chat_message_ids,
          ].slice(0, MAX_NOTIFIED_CHAT_MESSAGE_IDS);
          return {
            chat_badge_count: count_chat_unread_total(next_counts),
            chat_unread_counts: next_counts,
            notified_chat_message_ids: next_message_ids,
          };
        });
        return did_record;
      },
      clear_chat_notifications_for_target: (target_key) => set((state) => {
        const normalized_target_key = target_key?.trim();
        if (!normalized_target_key || !state.chat_unread_counts[normalized_target_key]) {
          return state;
        }
        const next_counts = { ...state.chat_unread_counts };
        delete next_counts[normalized_target_key];
        return {
          chat_badge_count: count_chat_unread_total(next_counts),
          chat_unread_counts: next_counts,
        };
      }),

      set_wide_panel_width: (width) =>
        set({ wide_panel_width: clamp_panel_width(width) }),

      toggle_section: (section_id) =>
        set((state) => ({
          collapsed_sections: {
            ...state.collapsed_sections,
            [section_id]: !state.collapsed_sections[section_id],
          },
        })),
    }),
    {
      name: "nexus-sidebar",
      // 只持久化布局相关状态，条目高亮保持运行时态
      partialize: (state) => ({
        wide_panel_width: state.wide_panel_width,
        collapsed_sections: state.collapsed_sections,
      }),
    },
  ),
);
