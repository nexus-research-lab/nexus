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
import { Link, useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_default_agent_id, is_main_agent } from "@/config/options";
import { CapabilitiesPanelContent } from "@/features/capability/capabilities-sidebar-panel";
import { HomePanelContent } from "@/features/home/home-sidebar-panel";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn, get_icon_avatar_src, get_initials } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { CollapsibleSection } from "@/shared/ui/sidebar/collapsible-section";
import { GlassMagnifierStatic } from "@/shared/ui/liquid-glass";
import { COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import { useAgentStore } from "@/store/agent";
import {
  derive_sidebar_item_id_from_path,
  SIDEBAR_SYSTEM_ITEM_IDS,
  useSidebarStore,
} from "@/store/sidebar";

const CAPABILITY_SECTION_COUNT = 5;
const SIDEBAR_RESIZE_HOTZONE_WIDTH = 8;
const MODAL_ROOT_SELECTOR = "[data-modal-root='true']";

export function SidebarWidePanel() {
  const { t } = useI18n();
  const { logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const nexus_room_id = useSidebarStore((s) => s.nexus_room_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);
  const root_ref = useRef<HTMLDivElement | null>(null);
  const [is_resize_hotzone_active, set_is_resize_hotzone_active] = useState(false);
  const is_settings_route = location.pathname.startsWith(AppRouteBuilders.settings());
  const prefers_reduced_motion = usePrefersReducedMotion();
  const default_agent_id = get_default_agent_id();
  const nexus_agent = agents.find((agent) => is_main_agent(agent.agent_id)) ?? null;
  const nexus_avatar_src = get_icon_avatar_src(nexus_agent?.avatar);
  const nexus_initials = get_initials(nexus_agent?.name, "NX", 2);
  const is_nexus_active = active_panel_item_id === SIDEBAR_SYSTEM_ITEM_IDS.nexus
    || (nexus_room_id ? active_panel_item_id === nexus_room_id : false);

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

  const handle_open_nexus = useCallback(() => {
    if (!default_agent_id) {
      return;
    }

    set_active_panel_item(SIDEBAR_SYSTEM_ITEM_IDS.nexus);
    void resolve_direct_room_navigation_target(default_agent_id).then(({ route }) => {
      navigate(route);
    }).catch((error) => {
      console.error("[SidebarWidePanel] 打开 Nexus DM 失败:", error);
    });
  }, [default_agent_id, navigate, set_active_panel_item]);

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
        <button
          className="group/nexus relative flex h-12 w-[68px] shrink-0 items-center justify-center"
          onClick={handle_open_nexus}
          title="Nexus"
          type="button"
        >
          <GlassMagnifierStatic
            class_name={cn(
              "relative z-10 transition-transform duration-(--motion-duration-normal)",
              !prefers_reduced_motion && "group-hover/nexus:scale-[1.03]",
              is_nexus_active && "drop-shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_12%,transparent)]",
            )}
            height={38}
            underlay={is_nexus_active ? (
              <>
                {/* 中文注释：把圆形彩光作为玻璃组件的下层内容，保证折射和高光都基于真实下层，而不是页面层假叠加。 */}
                <span
                  className={cn(
                    "absolute left-1/2 top-1/2 h-[36px] w-[36px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-88 blur-[0.5px]",
                    !prefers_reduced_motion && "animate-[spin_5.2s_linear_infinite]",
                  )}
                  style={{
                    background: "conic-gradient(from 180deg, transparent 0deg, transparent 24deg, rgba(96,165,250,0.98) 58deg, rgba(167,139,250,0.92) 104deg, transparent 146deg, transparent 206deg, rgba(52,211,153,0.9) 240deg, rgba(245,158,11,0.92) 280deg, rgba(244,114,182,0.94) 320deg, transparent 348deg, transparent 360deg)",
                    WebkitMask: "radial-gradient(farthest-side, transparent calc(100% - 3px), #000 calc(100% - 1px))",
                    mask: "radial-gradient(farthest-side, transparent calc(100% - 3px), #000 calc(100% - 1px))",
                  }}
                />
                <span
                  className={cn(
                    "absolute left-1/2 top-1/2 h-[28px] w-[28px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-48 blur-[8px]",
                    !prefers_reduced_motion && "animate-[spin_8.6s_linear_infinite_reverse]",
                  )}
                  style={{
                    background: "conic-gradient(from 180deg, transparent 0deg, rgba(96,165,250,0.84) 66deg, transparent 136deg, transparent 214deg, rgba(244,114,182,0.82) 292deg, rgba(52,211,153,0.74) 336deg, transparent 360deg)",
                  }}
                />
                <span className="absolute left-1/2 top-1/2 h-[24px] w-[24px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[radial-gradient(circle_at_34%_28%,rgba(255,255,255,0.34),transparent_42%),radial-gradient(circle_at_68%_72%,rgba(255,255,255,0.14),transparent_48%)] opacity-82 blur-[3px]" />
              </>
            ) : undefined}
            width={58}
          >
            <span className="relative flex h-8 w-8 items-center justify-center">
              <span
                className={cn(
                  "relative z-10 flex h-8 w-8 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
                  is_nexus_active && "shadow-[0_0_0_1px_rgba(255,255,255,0.14),0_0_10px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
                )}
              >
                {is_nexus_active ? (
                  <>
                    {/* 中文注释：这一层只做很轻的玻璃反光，不再承担主动画；主动态来自下层彩光被玻璃折射。 */}
                    <span className="pointer-events-none absolute inset-0 z-20 rounded-full bg-[radial-gradient(circle_at_28%_24%,rgba(255,255,255,0.24),transparent_38%),linear-gradient(132deg,rgba(255,255,255,0.18),transparent_42%,transparent_60%,rgba(255,255,255,0.08))] mix-blend-screen opacity-72" />
                    <span className="pointer-events-none absolute inset-[1px] z-20 rounded-full border border-[rgba(255,255,255,0.22)] opacity-72" />
                  </>
                ) : null}
                {nexus_avatar_src ? (
                  <img
                    alt="Nexus"
                    className="relative z-10 h-full w-full object-cover"
                    src={nexus_avatar_src}
                  />
                ) : (
                  <span className="relative z-10 text-[11px] font-semibold tracking-[0.08em] text-primary">
                    {nexus_initials}
                  </span>
                )}
              </span>
            </span>
          </GlassMagnifierStatic>
        </button>
        <div className="min-w-0">
          <Link
            className="block transition-transform duration-(--motion-duration-normal) hover:translate-y-[-0.5px]"
            title={t("sidebar.back_to_launcher")}
            to={AppRouteBuilders.launcher()}
          >
            <p
              className="text-[24px] uppercase tracking-[0.14em]"
              style={{
                fontFamily: "\"Panchang\", var(--font-sans)",
                fontWeight: 200,
              }}
            >
              NEXUS
            </p>
          </Link>
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
