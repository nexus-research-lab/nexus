/**
 * 侧边栏宽面板（可拖拽调整宽度）
 *
 * 保留原来的内容面板，并把一级导航压缩到头部，
 * 这样左侧不再需要独立的窄栏。
 *
 * 面板根据当前路由切换内容，并支持拖拽调整宽度。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 */

import { Settings } from "lucide-react";
import { useCallback, useEffect, useRef } from "react";
import { Link, useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { LanguageSwitch } from "@/shared/ui/i18n/language-switch";
import { CollapsibleSection } from "@/shared/ui/sidebar/collapsible-section";
import { ThemeSwitch } from "@/shared/ui/theme/theme-switch";
import { WorkspaceIconFrame } from "@/shared/ui/workspace/workspace-catalog-card";
import {
  derive_sidebar_item_id_from_path,
  WIDE_PANEL_MIN_WIDTH,
  WIDE_PANEL_MAX_WIDTH,
  useSidebarStore,
} from "@/store/sidebar";

import { HomePanelContent } from "./sidebar-panel-content/home-panel";
import { CapabilitiesPanelContent } from "./sidebar-panel-content/capabilities-panel";

const CAPABILITY_SECTION_COUNT = 5;

export function SidebarWidePanel() {
  const { t } = useI18n();
  const location = useLocation();
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);

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

  /** 路由变化时统一同步侧栏高亮，避免能力和房间走两套状态。 */
  useEffect(() => {
    const next_active_item_id = derive_sidebar_item_id_from_path(location.pathname);
    if (next_active_item_id === active_panel_item_id) {
      return;
    }
    set_active_panel_item(next_active_item_id);
  }, [active_panel_item_id, location.pathname, set_active_panel_item]);

  return (
    <div
      className="relative flex h-full shrink-0 flex-col px-2.5 py-3"
      style={{ width: wide_panel_width }}
    >
      <div className="surface-panel radius-shell-xl flex h-full w-full flex-col overflow-hidden">
        {/* 面板头部 */}
        <div className="border-b glass-divider px-4 py-2.5">
          <div className="flex items-center gap-2.5">
            <div className="flex min-w-0 items-center gap-2.5">
              <Link
                className="shrink-0 transition-transform duration-200 hover:translate-y-[-0.5px]"
                to={AppRouteBuilders.launcher()}
                title={t("sidebar.back_to_launcher")}
              >
                <WorkspaceIconFrame class_name="text-[15px] font-black tracking-[-0.06em] text-slate-900" size="sm">
                  N
                </WorkspaceIconFrame>
              </Link>
              <div className="min-w-0">
                <p className="text-[10px] font-medium uppercase tracking-[0.22em] text-slate-500/68">
                  {t("sidebar.workspace_label")}
                </p>
                <h2 className="truncate text-[15px] font-bold tracking-[-0.03em] text-slate-800">{t("sidebar.workspace_title")}</h2>
              </div>
            </div>
          </div>
        </div>

        {/* 面板内容 */}
        <div className="flex-1 overflow-y-auto px-2.5 py-2">
          <HomePanelContent />

          <CollapsibleSection
            count={CAPABILITY_SECTION_COUNT}
            section_id="sidebar-capabilities"
            title={t("sidebar.capabilities")}
          >
            <CapabilitiesPanelContent />
          </CollapsibleSection>
        </div>

        <div className="flex items-center justify-between gap-2.5 border-t glass-divider px-3 py-2.5">
          <Link
            className="transition-all duration-200 hover:text-slate-900"
            title={t("sidebar.settings")}
            to={AppRouteBuilders.settings()}
          >
            <WorkspaceIconFrame class_name="h-8 w-8 text-slate-600" size="sm">
              <Settings className="h-4 w-4" />
            </WorkspaceIconFrame>
          </Link>

          <div className="flex flex-wrap items-center justify-end gap-1.5">
            <ThemeSwitch />
            <LanguageSwitch />
          </div>
        </div>
      </div>

      {/* 右边缘拖拽手柄 */}
      <div
        className={cn(
          "absolute right-0 top-0 z-10 h-full w-1 cursor-col-resize",
          "transition-colors duration-150 hover:bg-slate-400/18",
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
