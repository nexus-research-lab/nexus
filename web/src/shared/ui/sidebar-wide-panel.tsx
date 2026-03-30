/**
 * 侧边栏右侧宽面板（可拖拽调整宽度）
 *
 * 右侧宽面板：展示选中 Tab 的完整内容。
 * 根据 active_tab 切换不同的面板内容组件。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 *
 * Phase 1a：先渲染空容器 + Tab 标题，验证布局。
 * Phase 1b：填充各 Tab 的面板内容组件。
 */

import { useCallback, useEffect, useRef } from "react";
import { useLocation } from "react-router-dom";

import { cn } from "@/lib/utils";
import {
  type SidebarTabKey,
  type SidebarCollapseMode,
  WIDE_PANEL_MIN_WIDTH,
  WIDE_PANEL_MAX_WIDTH,
  derive_tab_from_path,
  useSidebarStore,
} from "@/store/sidebar";

import { HomePanelContent } from "./sidebar-panel-content/home-panel";
import { DmsPanelContent } from "./sidebar-panel-content/dms-panel";
import { ActivityPanelContent } from "./sidebar-panel-content/activity-panel";
import { CapabilitiesPanelContent } from "./sidebar-panel-content/capabilities-panel";
import { ContactsPanelContent } from "./sidebar-panel-content/contacts-panel";

/** Tab 标题映射 */
const TAB_TITLES: Record<SidebarTabKey, string> = {
  home: "工作台",
  dms: "私信",
  activity: "活动",
  capabilities: "能力",
  contacts: "成员",
};

/** 根据折叠模式判断是否显示宽面板 */
function should_show_panel(mode: SidebarCollapseMode): boolean {
  return mode === "full";
}

/** 根据激活 Tab 渲染对应的面板内容 */
function render_panel_content(tab: SidebarTabKey) {
  switch (tab) {
    case "home":
      return <HomePanelContent />;
    case "dms":
      return <DmsPanelContent />;
    case "activity":
      return <ActivityPanelContent />;
    case "capabilities":
      return <CapabilitiesPanelContent />;
    case "contacts":
      return <ContactsPanelContent />;
    default:
      return null;
  }
}

export function SidebarWidePanel() {
  const location = useLocation();
  const active_tab = useSidebarStore((s) => s.active_tab);
  const collapse_mode = useSidebarStore((s) => s.collapse_mode);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);

  const derived_tab = derive_tab_from_path(location.pathname);
  const current_tab = active_tab || derived_tab;

  /** 拖拽状态 ref，避免频繁 re-render */
  const is_dragging_ref = useRef(false);
  const start_x_ref = useRef(0);
  const start_width_ref = useRef(0);

  /** 拖拽开始 */
  const handle_pointer_down = useCallback(
    (e: React.PointerEvent) => {
      e.preventDefault();
      is_dragging_ref.current = true;
      start_x_ref.current = e.clientX;
      start_width_ref.current = wide_panel_width;
      // 捕获指针，确保拖拽到面板外也能响应
      (e.target as HTMLElement).setPointerCapture(e.pointerId);
    },
    [wide_panel_width],
  );

  /** 拖拽中实时更新宽度 */
  const handle_pointer_move = useCallback(
    (e: React.PointerEvent) => {
      if (!is_dragging_ref.current) return;
      const delta = e.clientX - start_x_ref.current;
      const next_width = start_width_ref.current + delta;
      // clamp 在 store action 中处理
      set_wide_panel_width(next_width);
    },
    [set_wide_panel_width],
  );

  /** 拖拽结束 */
  const handle_pointer_up = useCallback(() => {
    is_dragging_ref.current = false;
  }, []);

  /** 拖拽时禁止文本选中 */
  useEffect(() => {
    const handle_select_start = (e: Event) => {
      if (is_dragging_ref.current) e.preventDefault();
    };
    document.addEventListener("selectstart", handle_select_start);
    return () => document.removeEventListener("selectstart", handle_select_start);
  }, []);

  // 折叠模式下不显示宽面板
  if (!should_show_panel(collapse_mode)) {
    return null;
  }

  return (
    <div
      className="relative flex h-full shrink-0 flex-col py-4 pl-0 pr-2"
      style={{ width: wide_panel_width }}
    >
      <div className="home-glass-panel radius-shell-xl flex h-full w-full flex-col overflow-hidden">
        {/* 面板头部 */}
        <div className="flex items-center gap-2 border-b border-white/20 px-4 py-3">
          <h2 className="text-sm font-bold text-slate-800">
            {TAB_TITLES[current_tab]}
          </h2>
        </div>

        {/* 面板内容 */}
        <div className="flex-1 overflow-y-auto px-2 py-2">
          {render_panel_content(current_tab)}
        </div>
      </div>

      {/* 右边缘拖拽手柄 */}
      <div
        className={cn(
          "absolute right-0 top-0 z-10 h-full w-1 cursor-col-resize",
          "transition-colors duration-150 hover:bg-slate-400/30",
        )}
        onPointerDown={handle_pointer_down}
        onPointerMove={handle_pointer_move}
        onPointerUp={handle_pointer_up}
        role="separator"
        aria-orientation="vertical"
        aria-valuemin={WIDE_PANEL_MIN_WIDTH}
        aria-valuemax={WIDE_PANEL_MAX_WIDTH}
        aria-valuenow={wide_panel_width}
        tabIndex={0}
      />
    </div>
  );
}
