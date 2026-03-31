/**
 * 侧边栏状态 Store
 *
 * 管理双面板侧边栏的 Tab 激活、面板条目高亮、折叠模式等状态。
 * 双面板设计：左侧窄栏（Tab 选择）+ 右侧宽面板（Tab 内容）。
 */

import { create } from "zustand";
import { persist } from "zustand/middleware";

// Module-level timer so we can cancel it before creating a new one.
// Avoids stacking multiple timeouts when set_navigated_from_tab is called rapidly.
let _navigated_from_tab_timer: ReturnType<typeof setTimeout> | null = null;

/** 侧边栏 Tab 类型 */
export type SidebarTabKey =
  | "home"
  | "dms"
  | "activity"
  | "capabilities"
  | "contacts";

/** 侧边栏折叠模式 */
export type SidebarCollapseMode =
  | "full"          // 全展开：窄栏 88px + 宽面板 240px
  | "narrow-only"   // 仅窄栏：88px
  | "icon-only"     // 纯图标：56px
  | "collapsed";    // 全收起：0px

/** 宽面板宽度约束 */
export const WIDE_PANEL_MIN_WIDTH = 180;
export const WIDE_PANEL_MAX_WIDTH = 400;
export const WIDE_PANEL_DEFAULT_WIDTH = 240;

/** 将宽度限制在合法范围内 */
function clamp_panel_width(width: number): number {
  return Math.round(Math.min(WIDE_PANEL_MAX_WIDTH, Math.max(WIDE_PANEL_MIN_WIDTH, width)));
}

interface SidebarState {
  /** 当前激活的 Tab */
  active_tab: SidebarTabKey;
  /** 宽面板中当前高亮的条目 ID（Room/DM/Agent/Skill） */
  active_panel_item_id: string | null;
  /** 折叠模式 */
  collapse_mode: SidebarCollapseMode;
  /** 宽面板宽度（px），支持拖拽调整 */
  wide_panel_width: number;
  /** 宽面板各 Section 的折叠状态 */
  collapsed_sections: Record<string, boolean>;
  /** 导航来源 Tab（用于 DM 对话页判断激活 Tab） */
  navigated_from_tab: SidebarTabKey | null;
}

interface SidebarActions {
  set_active_tab: (tab: SidebarTabKey) => void;
  set_active_panel_item: (id: string | null) => void;
  set_collapse_mode: (mode: SidebarCollapseMode) => void;
  /** 设置宽面板宽度，自动 clamp 到 [180, 400] */
  set_wide_panel_width: (width: number) => void;
  toggle_section: (section_id: string) => void;
  set_navigated_from_tab: (tab: SidebarTabKey | null) => void;
}

/**
 * 根据路由路径推导激活的 Tab
 * 用于 URL 直接访问时自动设置正确的 Tab
 */
export function derive_tab_from_path(pathname: string): SidebarTabKey {
  if (pathname.startsWith("/dms")) return "dms";
  if (pathname.startsWith("/activity")) return "activity";
  if (
    pathname.startsWith("/capability/")
  ) return "capabilities";
  if (pathname.startsWith("/contacts")) return "contacts";
  // /app、/rooms、以及默认情况都归到 home
  return "home";
}

export const useSidebarStore = create<SidebarState & SidebarActions>()(
  persist(
    (set) => ({
      active_tab: "home",
      active_panel_item_id: null,
      collapse_mode: "full" as SidebarCollapseMode,
      wide_panel_width: WIDE_PANEL_DEFAULT_WIDTH,
      collapsed_sections: {},
      navigated_from_tab: null,

      set_active_tab: (tab) => set({ active_tab: tab }),

      set_active_panel_item: (id) => set({ active_panel_item_id: id }),

      set_collapse_mode: (mode) => set({ collapse_mode: mode }),

      set_wide_panel_width: (width) =>
        set({ wide_panel_width: clamp_panel_width(width) }),

      toggle_section: (section_id) =>
        set((state) => ({
          collapsed_sections: {
            ...state.collapsed_sections,
            [section_id]: !state.collapsed_sections[section_id],
          },
        })),

      set_navigated_from_tab: (tab) => {
        if (_navigated_from_tab_timer !== null) {
          clearTimeout(_navigated_from_tab_timer);
          _navigated_from_tab_timer = null;
        }
        set({ navigated_from_tab: tab });
      },
    }),
    {
      name: "nexus-sidebar",
      // 只持久化折叠相关状态和宽面板宽度，其余随路由推导
      partialize: (state) => ({
        collapse_mode: state.collapse_mode,
        wide_panel_width: state.wide_panel_width,
        collapsed_sections: state.collapsed_sections,
      }),
    },
  ),
);
