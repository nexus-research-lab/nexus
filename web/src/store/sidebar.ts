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

  if (pathname.startsWith("/contacts/")) {
    const agent_id = pathname.split("/")[2];
    return agent_id ? decodeURIComponent(agent_id) : null;
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
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
}

interface SidebarActions {
  set_active_panel_item: (id: string | null) => void;
  set_nexus_room_id: (room_id: string | null) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  toggle_section: (section_id: string) => void;
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_panel_item_id: null,
      nexus_room_id: null,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      collapsed_sections: {},

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),
      set_nexus_room_id: (room_id) => set({ nexus_room_id: room_id }),

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
