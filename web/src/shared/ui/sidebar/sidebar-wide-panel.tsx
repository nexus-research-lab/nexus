/**
 * 侧边栏宽面板（可拖拽调整宽度）
 *
 * 保留原来的内容面板，并把一级导航压缩到头部，
 * 这样左侧不再需要独立的窄栏。
 *
 * 面板根据当前路由切换内容，并支持拖拽调整宽度。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 */

import { LogOut, Settings } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { CollapsibleSection } from "@/shared/ui/sidebar/collapsible-section";
import { GlassMagnifierStatic } from "@/shared/ui/liquid-glass";
import { COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS } from "@/shared/ui/workspace/workspace-header-layout";
import {
  derive_sidebar_item_id_from_path,
  useSidebarStore,
} from "@/store/sidebar";

import { HomePanelContent } from "./sidebar-panel-content/home-panel";
import { CapabilitiesPanelContent } from "./sidebar-panel-content/capabilities-panel";

const CAPABILITY_SECTION_COUNT = 5;
const SIDEBAR_RESIZE_HOTZONE_WIDTH = 8;
const MODAL_ROOT_SELECTOR = "[data-modal-root='true']";

export function SidebarWidePanel() {
  const { t } = useI18n();
  const { logout } = useAuth();
  const location = useLocation();
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);
  const root_ref = useRef<HTMLDivElement | null>(null);
  const [is_resize_hotzone_active, set_is_resize_hotzone_active] = useState(false);
  const is_settings_route = location.pathname.startsWith(AppRouteBuilders.settings());

  /** 拖拽状态 ref，避免频繁 re-render */
  const is_dragging_ref = useRef(false);
  const start_x_ref = useRef(0);
  const start_width_ref = useRef(0);

  /** 拖拽开始 */
  const handle_pointer_down = useCallback(
    (e: React.PointerEvent) => {
      if (e.target instanceof HTMLElement && e.target.closest(MODAL_ROOT_SELECTOR)) {
        return;
      }
      const root_element = root_ref.current;
      if (!root_element) {
        return;
      }

      const rect = root_element.getBoundingClientRect();
      const distance_to_right_edge = rect.right - e.clientX;
      if (distance_to_right_edge > SIDEBAR_RESIZE_HOTZONE_WIDTH) {
        return;
      }

      e.preventDefault();
      is_dragging_ref.current = true;
      start_x_ref.current = e.clientX;
      start_width_ref.current = wide_panel_width;
      set_is_resize_hotzone_active(true);
      // 捕获指针，确保拖拽到面板外也能响应
      e.currentTarget.setPointerCapture(e.pointerId);
    },
    [wide_panel_width],
  );

  /** 拖拽中实时更新宽度 */
  const handle_pointer_move = useCallback(
    (e: React.PointerEvent) => {
      if (e.target instanceof HTMLElement && e.target.closest(MODAL_ROOT_SELECTOR)) {
        if (!is_dragging_ref.current) {
          set_is_resize_hotzone_active(false);
        }
        return;
      }
      const root_element = root_ref.current;
      if (!root_element) {
        return;
      }

      if (!is_dragging_ref.current) {
        const rect = root_element.getBoundingClientRect();
        const distance_to_right_edge = rect.right - e.clientX;
        set_is_resize_hotzone_active(distance_to_right_edge <= SIDEBAR_RESIZE_HOTZONE_WIDTH);
        return;
      }

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
    set_is_resize_hotzone_active(false);
  }, []);

  /** 离开热区时恢复默认光标，避免残留 resize 态。 */
  const handle_pointer_leave = useCallback(() => {
    if (is_dragging_ref.current) {
      return;
    }
    set_is_resize_hotzone_active(false);
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
      className={cn(
        "desktop-rail relative flex h-full shrink-0 flex-col",
        HOME_SIDEBAR_PADDING_CLASS,
        is_resize_hotzone_active && "cursor-col-resize",
      )}
      onPointerDown={handle_pointer_down}
      onPointerLeave={handle_pointer_leave}
      onPointerMove={handle_pointer_move}
      onPointerUp={handle_pointer_up}
      ref={root_ref}
      style={{ width: wide_panel_width }}
    >
      {/* 面板头部 */}
      <div className={cn("flex items-center gap-3 border-b divider-subtle px-4", COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS)}>
        <Link
          className="shrink-0 transition-transform duration-(--motion-duration-normal) hover:translate-y-[-0.5px]"
          to={AppRouteBuilders.launcher()}
          title={t("sidebar.back_to_launcher")}
        >
          <GlassMagnifierStatic
            class_name="translate-y-[1px]"
            height={34}
            width={58}
          >
            <span className="text-[24px] font-black tracking-[-0.08em] text-primary ">N</span>
          </GlassMagnifierStatic>
        </Link>
        <div className="min-w-0">
          <p className="text-[26px] font-semibold uppercase tracking-[0.24em] text-(--text-strong)">
            NEXUS
          </p>
        </div>
      </div>

      {/* 面板内容 */}
      <div className="soft-scrollbar scrollbar-stable-gutter flex-1 overflow-y-auto px-2.5 py-2.5">
        <HomePanelContent />

        <CollapsibleSection
          count={CAPABILITY_SECTION_COUNT}
          section_id="sidebar-capabilities"
          title={t("sidebar.capabilities")}
        >
          <CapabilitiesPanelContent />
        </CollapsibleSection>
      </div>

      <div className="relative flex items-center justify-between gap-2.5 border-t divider-subtle px-3 py-3">
        <Link
          className={cn(
            "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
            is_settings_route && "bg-(--surface-interactive-active-background) text-(--text-strong)",
          )}
          title={t("sidebar.settings")}
          to={AppRouteBuilders.settings()}
        >
            <Settings className="h-4 w-4" />
        </Link>

        <div className="min-w-0 flex-1" />

        <button
          className="flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
          onClick={() => {
            void logout();
          }}
          title={t("sidebar.logout")}
          type="button"
        >
          <LogOut className="h-4 w-4" />
        </button>
      </div>
    </div>
  );
}
