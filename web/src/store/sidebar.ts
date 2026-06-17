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
type WidePanelCollapseSource = "manual" | "right_panel_auto";
export const SIDEBAR_SYSTEM_ITEM_IDS = {
  nexus: "system:nexus",
} as const;
export const SIDEBAR_CAPABILITY_ITEM_IDS = {
  skills: "capability:skills",
  loops: "capability:loops",
  connectors: "capability:connectors",
  scheduled_tasks: "capability:scheduled-tasks",
  channels: "capability:channels",
  pairings: "capability:pairings",
  memory: "capability:memory",
} as const;

/** 根据当前路由派生侧栏高亮条目，保证整套导航只走一个状态源。 */
export function derive_sidebar_item_id_from_path(pathname: string): string | null {
  if (pathname.startsWith("/capability/skills")) return SIDEBAR_CAPABILITY_ITEM_IDS.skills;
  if (pathname.startsWith("/capability/loops")) return SIDEBAR_CAPABILITY_ITEM_IDS.loops;
  if (pathname.startsWith("/capability/connectors")) return SIDEBAR_CAPABILITY_ITEM_IDS.connectors;
  if (pathname.startsWith("/capability/scheduled-tasks")) return SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks;
  if (pathname.startsWith("/capability/channels")) return SIDEBAR_CAPABILITY_ITEM_IDS.channels;
  if (pathname.startsWith("/capability/pairings")) return SIDEBAR_CAPABILITY_ITEM_IDS.pairings;
  if (pathname.startsWith("/memory")) return SIDEBAR_CAPABILITY_ITEM_IDS.memory;

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

export interface ChatNotificationTargetState {
  key: string;
  room_id?: string | null;
  conversation_id?: string | null;
  session_key?: string | null;
}

interface SidebarState {
  /** 宽面板中当前高亮的条目 ID（Room/DM/Agent/Skill） */
  active_panel_item_id: string | null;
  /** 主智能体 DM 的真实 room_id，用于 header 入口和真实 room 路由共用同一激活语义。 */
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
  set_nexus_room_id: (room_id: string | null) => void;
  set_chat_badge_count: (count: number) => void;
  record_chat_notification: (target: ChatNotificationTargetState, message_id: string) => boolean;
  clear_chat_notifications_for_target: (target_key: string | null | undefined) => void;
  clear_chat_notifications_for_room: (room_id: string | null | undefined) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  set_wide_panel_collapsed: (collapsed: boolean) => void;
  toggle_wide_panel_collapsed: () => void;
  collapse_wide_panel_for_right_panel: () => void;
  expand_wide_panel_after_right_panel: () => void;
  toggle_section: (section_id: string) => void;
}

const MAX_NOTIFIED_CHAT_MESSAGE_IDS = 300;

function count_chat_unread_total(counts: Record<string, number>): number {
  return Object.values(counts).reduce((total, count) => total + Math.max(0, count), 0);
}

function clear_chat_unread_keys(
  state: SidebarState,
  keys: string[],
): Pick<SidebarState, "chat_badge_count" | "chat_unread_counts" | "chat_unread_targets" | "chat_unread_timestamps"> {
  const unique_keys = Array.from(new Set(keys.filter(Boolean)));
  if (unique_keys.length === 0) {
    return {
      chat_badge_count: state.chat_badge_count,
      chat_unread_counts: state.chat_unread_counts,
      chat_unread_targets: state.chat_unread_targets,
      chat_unread_timestamps: state.chat_unread_timestamps,
    };
  }

  const next_counts = { ...state.chat_unread_counts };
  const next_targets = { ...state.chat_unread_targets };
  const next_timestamps = { ...state.chat_unread_timestamps };
  for (const key of unique_keys) {
    delete next_counts[key];
    delete next_targets[key];
    delete next_timestamps[key];
  }
  return {
    chat_badge_count: count_chat_unread_total(next_counts),
    chat_unread_counts: next_counts,
    chat_unread_targets: next_targets,
    chat_unread_timestamps: next_timestamps,
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
      set_nexus_room_id: (room_id) => set({ nexus_room_id: room_id }),
      set_chat_badge_count: (count) => set({ chat_badge_count: Math.max(0, Math.floor(count)) }),
      record_chat_notification: (target, message_id) => {
        let did_record = false;
        set((state) => {
          const normalized_target_key = target.key.trim();
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
          const next_targets = {
            ...state.chat_unread_targets,
            [normalized_target_key]: {
              ...target,
              key: normalized_target_key,
            },
          };
          const next_timestamps = {
            ...state.chat_unread_timestamps,
            [normalized_target_key]: Date.now(),
          };
          const next_message_ids = [
            normalized_message_id,
            ...state.notified_chat_message_ids,
          ].slice(0, MAX_NOTIFIED_CHAT_MESSAGE_IDS);
          return {
            chat_badge_count: count_chat_unread_total(next_counts),
            chat_unread_counts: next_counts,
            chat_unread_targets: next_targets,
            chat_unread_timestamps: next_timestamps,
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
        return clear_chat_unread_keys(state, [normalized_target_key]);
      }),
      clear_chat_notifications_for_room: (room_id) => set((state) => {
        const normalized_room_id = room_id?.trim();
        if (!normalized_room_id) {
          return state;
        }
        const room_key = `room:${normalized_room_id}`;
        const room_conversation_key_prefix = `${room_key}:conversation:`;
        const keys = Object.entries(state.chat_unread_targets)
          .filter(([, target]) => target.room_id === normalized_room_id)
          .map(([key]) => key);
        for (const key of Object.keys(state.chat_unread_counts)) {
          if (key === room_key || key.startsWith(room_conversation_key_prefix)) {
            keys.push(key);
          }
        }
        if (keys.length === 0) {
          return state;
        }
        return clear_chat_unread_keys(state, keys);
      }),

      set_wide_panel_width: (width) =>
        set({ wide_panel_width: clamp_panel_width(width) }),
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
