/**
 * 侧边栏宽面板（可拖拽调整宽度）
 *
 * 保留原来的内容面板，并把一级导航压缩到头部，
 * 这样左侧不再需要独立的窄栏。
 *
 * 面板根据当前路由切换内容，并支持拖拽调整宽度。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 */

import { ChevronRight, LogOut, Settings } from "lucide-react";
import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useLocation } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/home-layout";
import { cn } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
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
  const { logout } = useAuth();
  const location = useLocation();
  const [settings_open, set_settings_open] = useState(false);
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);
  const settings_popover_ref = useRef<HTMLDivElement | null>(null);

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

  /** 点击外部时关闭设置弹层。 */
  useEffect(() => {
    if (!settings_open) {
      return;
    }

    const handle_pointer_down = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) {
        return;
      }
      if (settings_popover_ref.current?.contains(event.target)) {
        return;
      }
      set_settings_open(false);
    };

    const handle_key_down = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        set_settings_open(false);
      }
    };

    document.addEventListener("pointerdown", handle_pointer_down);
    document.addEventListener("keydown", handle_key_down);
    return () => {
      document.removeEventListener("pointerdown", handle_pointer_down);
      document.removeEventListener("keydown", handle_key_down);
    };
  }, [settings_open]);

  return (
    <div
      className={cn("relative flex h-full shrink-0 flex-col", HOME_SIDEBAR_PADDING_CLASS)}
      style={{ width: wide_panel_width }}
    >
      <div className="surface-panel radius-shell-xl flex h-full w-full flex-col overflow-hidden">
        {/* 面板头部 */}
        <div className="flex items-center gap-2.5 border-b glass-divider px-4 py-2.5">
          <Link
            className="shrink-0 transition-transform duration-200 hover:translate-y-[-0.5px]"
            to={AppRouteBuilders.launcher()}
            title={t("sidebar.back_to_launcher")}
          >
            <WorkspaceIconFrame class_name="text-[15px] font-black tracking-[-0.06em] text-[color:var(--text-strong)]" size="sm">
              N
            </WorkspaceIconFrame>
          </Link>
          <div className="min-w-0">
            <p className="text-[11px] font-medium uppercase tracking-[0.22em] text-[color:var(--text-soft)]">
              {t("sidebar.workspace_label")}
            </p>
            <h2 className="truncate text-[16px] font-bold tracking-[-0.03em] text-[color:var(--text-strong)]">{t("sidebar.workspace_title")}</h2>
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

        <div className="relative flex items-center justify-between gap-2.5 border-t glass-divider px-3 py-2.5">
          <div className="relative" ref={settings_popover_ref}>
            <button
              aria-expanded={settings_open}
              aria-haspopup="dialog"
              className="transition-all duration-200 hover:text-[color:var(--text-strong)]"
              onClick={() => set_settings_open((open) => !open)}
              title={t("sidebar.settings")}
              type="button"
            >
              <WorkspaceIconFrame class_name="h-8 w-8 text-[color:var(--icon-default)]" size="sm">
                <Settings className="h-4 w-4" />
              </WorkspaceIconFrame>
            </button>

            {settings_open ? (
              <div
                className="absolute bottom-[calc(100%+8px)] left-0 z-20 w-[min(220px,calc(100vw-28px))] overflow-hidden rounded-[16px] border bg-[var(--surface-popover-background)] p-1.5 mb-2"
                style={{
                  borderColor: "var(--surface-popover-border)",
                }}
              >
                <Link
                  className="flex items-center justify-between rounded-[12px] px-2.5 py-2 text-[13px] font-medium text-[color:var(--text-default)] transition duration-150 ease-out hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]"
                  onClick={() => set_settings_open(false)}
                  to={AppRouteBuilders.settings()}
                >
                  <span>{t("sidebar.settings")}</span>
                  <ChevronRight className="h-4 w-4 text-[color:var(--icon-default)]" />
                </Link>

                <div className="mt-1.5 border-t pt-1.5" style={{ borderColor: "var(--divider-subtle-color)" }}>
                  <div className="px-1">
                    <p className="px-1.5 pb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-[color:var(--text-soft)]">
                      {t("theme.switch_title")}
                    </p>
                    <ThemeSwitch class_name="w-full" density="compact" stretch />
                  </div>

                  <div className="mt-1.5 px-1">
                    <p className="px-1.5 pb-1 text-[11px] font-medium uppercase tracking-[0.16em] text-[color:var(--text-soft)]">
                      {t("language.switch_title")}
                    </p>
                    <LanguageSwitch class_name="w-full" density="compact" show_icon={false} stretch />
                  </div>

                  <div className="mt-1.5 border-t px-1 pt-1.5" style={{ borderColor: "var(--divider-subtle-color)" }}>
                    <button
                      className="flex w-full items-center justify-between rounded-[12px] px-2.5 py-2 text-[13px] font-medium text-[color:var(--text-default)] transition duration-150 ease-out hover:bg-[var(--surface-interactive-hover-background)] hover:text-[color:var(--text-strong)]"
                      onClick={() => {
                        set_settings_open(false);
                        void logout();
                      }}
                      type="button"
                    >
                      <span>{t("sidebar.logout")}</span>
                      <LogOut className="h-4 w-4 text-[color:var(--icon-default)]" />
                    </button>
                  </div>
                </div>
              </div>
            ) : null}
          </div>

          <div className="min-w-0 flex-1" />
        </div>
      </div>

      {/* 右边缘拖拽手柄 */}
      <div
        className={cn(
          "absolute right-0 top-0 z-10 h-full w-1 cursor-col-resize",
          "transition-colors duration-150 hover:bg-[var(--divider-subtle-color)]",
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
