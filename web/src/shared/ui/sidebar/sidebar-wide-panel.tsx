/**
 * 侧边栏宽面板（可拖拽调整宽度）
 *
 * 保留原来的内容面板，并把一级导航压缩到头部，
 * 这样左侧不再需要独立的窄栏。
 *
 * 面板根据当前路由切换内容，并支持拖拽调整宽度。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 */

import { FolderOpen, Settings } from "lucide-react";
import { useCallback, useEffect, useRef } from "react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { cn } from "@/lib/utils";
import { CollapsibleSection } from "@/shared/ui/sidebar/collapsible-section";
import { WIDE_PANEL_MIN_WIDTH, WIDE_PANEL_MAX_WIDTH, useSidebarStore } from "@/store/sidebar";

import { HomePanelContent } from "./sidebar-panel-content/home-panel";
import { CapabilitiesPanelContent } from "./sidebar-panel-content/capabilities-panel";

const CAPABILITY_SECTION_COUNT = 5;

interface UtilityLink {
  label: string;
  to?: string;
  icon: typeof FolderOpen;
}

const UTILITY_LINKS: UtilityLink[] = [
  {
    label: "设置",
    to: AppRouteBuilders.settings(),
    icon: Settings,
  },
];

export function SidebarWidePanel() {
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

  return (
    <div
      className="relative flex h-full shrink-0 flex-col px-2 py-4"
      style={{ width: wide_panel_width }}
    >
      <div className="home-glass-panel radius-shell-xl flex h-full w-full flex-col overflow-hidden">
        {/* 面板头部 */}
        <div className="border-b border-white/20 px-4 py-3">
          <div className="flex items-center justify-between gap-3">
            <div className="flex min-w-0 items-center gap-3">
              <Link
                className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[16px] bg-[linear-gradient(180deg,rgba(255,255,255,0.92),rgba(233,229,223,0.92))] text-base font-black tracking-[-0.06em] text-slate-900 shadow-[0_12px_24px_rgba(102,112,145,0.12)] transition-transform duration-200 hover:scale-[1.03]"
                to={AppRouteBuilders.launcher()}
                title="回到 Launcher"
              >
                N
              </Link>
              <div className="min-w-0">
                <p className="text-[11px] font-medium uppercase tracking-[0.24em] text-slate-500/70">
                  Workspace
                </p>
                <h2 className="truncate text-sm font-bold text-slate-800">工作台</h2>
              </div>
            </div>

            <div className="flex items-center gap-1">
              {UTILITY_LINKS.map((item) => {
                const Icon = item.icon;
                return (
                  <Link
                    key={item.label}
                    className="flex h-9 w-9 items-center justify-center rounded-[14px] bg-white/30 text-slate-600 transition-all duration-200 hover:bg-white/55 hover:text-slate-900"
                    title={item.label}
                    to={item.to ?? AppRouteBuilders.home()}
                  >
                    <Icon className="h-4 w-4" />
                  </Link>
                );
              })}
            </div>
          </div>

        </div>

        {/* 面板内容 */}
        <div className="flex-1 overflow-y-auto px-2 py-2">
          <HomePanelContent />

          <CollapsibleSection
            count={CAPABILITY_SECTION_COUNT}
            section_id="sidebar-capabilities"
            title="能力"
          >
            <CapabilitiesPanelContent />
          </CollapsibleSection>
        </div>

        <div className="flex items-center gap-2 border-t border-white/16 px-3 py-2">
          <a
            className="rounded-full px-2 py-1 text-[11px] font-medium text-slate-500 transition-colors hover:bg-white/35 hover:text-slate-900"
            href="https://docs.nexus.ai"
            rel="noreferrer"
            target="_blank"
          >
            文档
          </a>
          <a
            className="rounded-full px-2 py-1 text-[11px] font-medium text-slate-500 transition-colors hover:bg-white/35 hover:text-slate-900"
            href="https://feedback.nexus.ai"
            rel="noreferrer"
            target="_blank"
          >
            反馈
          </a>
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
