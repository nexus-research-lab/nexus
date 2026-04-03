/**
 * 侧边栏状态 Store
 *
 * 当前侧栏只保留宽面板本体，
 * 这里集中管理列表高亮、分区折叠和面板宽度。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

/** 宽面板宽度约束 */
export const WIDE_PANEL_MIN_WIDTH = 180;
export const WIDE_PANEL_MAX_WIDTH = 400;
export const WIDE_PANEL_DEFAULT_WIDTH = 240;

/** 将宽度限制在合法范围内 */
function clamp_panel_width(width: number): number {
  return Math.round(Math.min(WIDE_PANEL_MAX_WIDTH, Math.max(WIDE_PANEL_MIN_WIDTH, width)));
}

interface SidebarState {
  /** 宽面板中当前高亮的条目 ID（Room/DM/Agent/Skill） */
  active_panel_item_id: string | null;
  /** 宽面板宽度（px），支持拖拽调整 */
  wide_panel_width: number;
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
}

interface SidebarActions {
  set_active_panel_item: (id: string | null) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  toggle_section: (section_id: string) => void;
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_panel_item_id: null,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      collapsed_sections: {},

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),

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
