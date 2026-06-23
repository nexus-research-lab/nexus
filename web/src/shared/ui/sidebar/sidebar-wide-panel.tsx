/**
 * 侧边栏宽面板（可拖拽调整宽度）
 *
 * 保留原来的内容面板，并把一级导航压缩到头部，
 * 这样左侧不再需要独立的窄栏。
 *
 * 面板根据当前路由切换内容，并支持拖拽调整宽度。
 * 宽度从 store 读取，右边缘可拖拽调整（180–400px）。
 */

import {
  Compass,
  LogOut,
  MessageCircle,
  PanelLeftClose,
  PanelLeftOpen,
  Puzzle,
  Settings,
  type LucideIcon,
  Users2,
} from "lucide-react";
import { useCallback, useEffect, useMemo } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_default_agent_avatar, get_default_agent_id, is_main_agent } from "@/config/options";
import { is_desktop_runtime } from "@/config/desktop-runtime";
import { CapabilitiesPanelContent } from "@/features/capability/capabilities-sidebar-panel";
import {
  ChatSidebarPanelContent,
  ContactsSidebarPanelContent,
} from "@/features/home/home-sidebar-panel";
import { useChatCompletionNotifications } from "@/features/home/use-chat-completion-notifications";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { resolve_direct_room_navigation_target } from "@/lib/conversation/direct-room-navigation";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn, get_icon_avatar_src } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiCounterBadge } from "@/shared/ui/badge";
import { OnboardingGuideCenter } from "@/shared/ui/onboarding/onboarding-guide-center";
import {
  SIDEBAR_TOUR_ANCHORS,
} from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useSidebarGuideCenter } from "@/shared/ui/sidebar/use-sidebar-guide-center";
import { useSidebarPanelResize } from "@/shared/ui/sidebar/use-sidebar-panel-resize";

import { GlassMagnifierStatic } from "@/shared/ui/liquid-glass";
import { COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import { useAgentStore } from "@/store/agent";
import {
  SIDEBAR_CAPABILITY_ITEM_IDS,
  derive_sidebar_item_id_from_path,
  SIDEBAR_SYSTEM_ITEM_IDS,
  useSidebarStore,
} from "@/store/sidebar";

type SidebarPrimaryTab = "chat" | "contacts" | "capabilities";

function derive_primary_tab_from_path(pathname: string): SidebarPrimaryTab {
  if (pathname.startsWith(AppRouteBuilders.contacts())) {
    return "contacts";
  }
  if (pathname.startsWith("/capability/") || pathname.startsWith(AppRouteBuilders.memory())) {
    return "capabilities";
  }
  return "chat";
}

export function SidebarWidePanel() {
  const { t } = useI18n();
  const { logout } = useAuth();
  const location = useLocation();
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const nexus_room_id = useSidebarStore((s) => s.nexus_room_id);
  const chat_badge_count = useSidebarStore((s) => s.chat_badge_count);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const wide_panel_width = useSidebarStore((s) => s.wide_panel_width);
  const set_wide_panel_width = useSidebarStore((s) => s.set_wide_panel_width);
  const wide_panel_collapsed = useSidebarStore((s) => s.wide_panel_collapsed);
  const set_wide_panel_collapsed = useSidebarStore((s) => s.set_wide_panel_collapsed);
  const toggle_wide_panel_collapsed = useSidebarStore((s) => s.toggle_wide_panel_collapsed);
  const is_settings_route = location.pathname.startsWith(AppRouteBuilders.settings());
  const active_primary_tab = derive_primary_tab_from_path(location.pathname);
  const should_show_logout = !is_desktop_runtime();
  const prefers_reduced_motion = usePrefersReducedMotion();
  const default_agent_id = get_default_agent_id();
  const nexus_agent = agents.find((agent) => is_main_agent(agent.agent_id)) ?? null;
  const nexus_avatar = nexus_agent?.avatar?.trim() || get_default_agent_avatar();
  const nexus_avatar_src = get_icon_avatar_src(nexus_avatar);
  const is_nexus_active = active_panel_item_id === SIDEBAR_SYSTEM_ITEM_IDS.nexus
    || (nexus_room_id ? active_panel_item_id === nexus_room_id : false);
  useChatCompletionNotifications();
  const {
    guide_center_props,
    is_guide_center_open,
    open_guide_center,
  } = useSidebarGuideCenter({
    default_agent_id,
    set_active_panel_item,
  });

  const {
    handle_pointer_down,
    handle_pointer_leave,
    handle_pointer_move,
    handle_pointer_up,
    is_resize_hotzone_active,
    root_ref,
  } = useSidebarPanelResize({
    set_wide_panel_width,
    wide_panel_width,
  });

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

  const handle_select_primary_tab = useCallback((tab: SidebarPrimaryTab) => {
    if (tab === "chat") {
      if (!location.pathname.startsWith("/rooms/")) {
        navigate(AppRouteBuilders.home());
      }
      return;
    }

    if (tab === "contacts") {
      set_active_panel_item(null);
      navigate(AppRouteBuilders.contacts());
      return;
    }

    set_active_panel_item(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
    navigate(AppRouteBuilders.skills());
  }, [location.pathname, navigate, set_active_panel_item]);

  const primary_tabs: {
    key: SidebarPrimaryTab;
    label: string;
    icon: LucideIcon;
    anchor: string;
    badge_count?: number;
  }[] = [
    {
      key: "chat",
      label: t("sidebar.tab_chat"),
      icon: MessageCircle,
      anchor: SIDEBAR_TOUR_ANCHORS.chat_tab,
      badge_count: active_primary_tab === "chat" ? 0 : chat_badge_count,
    },
    { key: "contacts", label: t("sidebar.tab_contacts"), icon: Users2, anchor: SIDEBAR_TOUR_ANCHORS.contacts_tab },
    { key: "capabilities", label: t("sidebar.tab_capabilities"), icon: Puzzle, anchor: SIDEBAR_TOUR_ANCHORS.capabilities_tab },
  ];

  if (wide_panel_collapsed) {
    return (
      <aside
        className={cn(
          "desktop-rail relative flex h-full w-[56px] shrink-0 flex-col items-center",
          HOME_SIDEBAR_PADDING_CLASS,
        )}
        data-sidebar-collapsed="true"
      >
        <div className="flex min-h-0 flex-1 flex-col items-center gap-2 pb-3 pt-2">
          <button
            aria-label="Nexus"
            className={cn(
              "flex h-9 w-9 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) text-[10px] font-semibold uppercase text-(--text-subtle) shadow-(--surface-avatar-shadow) transition-(transform,border-color,box-shadow) duration-(--motion-duration-fast) hover:-translate-y-px hover:border-(--surface-interactive-hover-border)",
              is_nexus_active && "border-(--surface-interactive-active-border) shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
            )}
            onClick={handle_open_nexus}
            title="Nexus"
            type="button"
          >
            {nexus_avatar_src ? (
              <img
                alt=""
                className="h-full w-full object-cover"
                src={nexus_avatar_src}
              />
            ) : "NX"}
          </button>

          <div className="mt-1 flex flex-col items-center gap-1.5">
            {primary_tabs.map((tab) => {
              const Icon = tab.icon;
              const is_active = active_primary_tab === tab.key;
              return (
                <button
                  aria-label={tab.label}
                  aria-current={is_active ? "page" : undefined}
                  className={cn(
                    "relative flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-(background,color,transform) duration-(--motion-duration-fast) hover:-translate-y-px hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                    is_active && "bg-(--surface-interactive-active-background) text-(--primary)",
                  )}
                  key={tab.key}
                  onClick={() => handle_select_primary_tab(tab.key)}
                  title={tab.label}
                  type="button"
                >
                  <Icon
                    className={cn(
                      "h-4 w-4",
                      is_active && "fill-(--primary) stroke-(--primary)",
                    )}
                  />
                  <UiCounterBadge
                    class_name="absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
                    count={tab.badge_count ?? 0}
                  />
                </button>
              );
            })}
          </div>
        </div>

        <div className="flex flex-col items-center gap-1.5 border-t divider-subtle py-3">
          <Link
            aria-label={t("sidebar.settings")}
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
              is_settings_route && "bg-(--surface-interactive-active-background) text-(--text-strong)",
            )}
            title={t("sidebar.settings")}
            to={AppRouteBuilders.settings()}
          >
            <Settings className="h-4 w-4" />
          </Link>

          <button
            aria-label={t("common.guide_center")}
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
              is_guide_center_open && "bg-(--surface-interactive-active-background) text-(--text-strong)",
            )}
            onClick={open_guide_center}
            title={t("common.guide_center")}
            type="button"
          >
            <Compass className="h-4 w-4" />
          </button>

          {should_show_logout ? (
            <button
              aria-label={t("sidebar.logout")}
              className="flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => {
                void logout();
              }}
              title={t("sidebar.logout")}
              type="button"
            >
              <LogOut className="h-4 w-4" />
            </button>
          ) : null}

          <button
            aria-label={t("sidebar.expand_panel")}
            className="flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
            onClick={toggle_wide_panel_collapsed}
            title={t("sidebar.expand_panel")}
            type="button"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </button>
        </div>

        <OnboardingGuideCenter
          {...guide_center_props}
        />
      </aside>
    );
  }

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
      <div className={cn(
        "grid grid-cols-[58px_minmax(0,1fr)] items-center gap-2 border-b divider-subtle px-2",
        COMPACT_WORKSPACE_HEADER_TOTAL_HEIGHT_CLASS,
      )}>
        <button
          className="group/nexus relative flex h-12 w-[58px] shrink-0 items-center justify-center"
          data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
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
                  <span className="relative z-10 text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-subtle)">NX</span>
                )}
              </span>
            </span>
          </GlassMagnifierStatic>
        </button>
        <div className="min-w-0">
          <Link
            className="block min-w-0 transition-transform duration-(--motion-duration-normal) hover:translate-y-[-0.5px]"
            data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
            title={t("sidebar.back_to_launcher")}
            to={AppRouteBuilders.launcher()}
          >
            <p
              className="whitespace-nowrap text-[21px] uppercase tracking-[0.08em]"
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

      {/* 一级 Tab：聊天、联系人、能力 */}
      <div className="border-b divider-subtle px-3 py-2">
        <div className="grid grid-cols-3 gap-1 rounded-[14px] bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_58%,transparent)] p-1">
          {primary_tabs.map((tab) => {
            const Icon = tab.icon;
            const is_active = active_primary_tab === tab.key;
            return (
              <button
                aria-current={is_active ? "page" : undefined}
                aria-pressed={is_active}
                className={cn(
                  "flex h-9 items-center justify-center gap-1.5 rounded-[11px] text-[13px] font-medium transition-[background,color,box-shadow] duration-(--motion-duration-fast)",
                  is_active
                    ? "bg-[color:color-mix(in_srgb,var(--primary)_14%,var(--surface-elevated-background))] text-(--primary) shadow-[0_8px_22px_color-mix(in_srgb,var(--primary)_10%,transparent)]"
                    : "text-(--text-muted) hover:text-(--text-strong)",
                )}
                data-tour-anchor={tab.anchor}
                key={tab.key}
                onClick={() => handle_select_primary_tab(tab.key)}
                type="button"
              >
                <span className="relative flex h-4 w-4 items-center justify-center">
                  <Icon
                    className={cn(
                      "h-3.5 w-3.5",
                      is_active && "fill-(--primary) stroke-(--primary)",
                    )}
                  />
                  <UiCounterBadge
                    class_name="absolute -right-2.5 -top-2 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
                    count={tab.badge_count ?? 0}
                  />
                </span>
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* 面板内容 */}
      <div className="soft-scrollbar scrollbar-stable-gutter flex min-h-0 flex-1 flex-col overflow-y-auto py-2.5">
        {active_primary_tab === "chat" ? (
          <ChatSidebarPanelContent />
        ) : null}

        {active_primary_tab === "contacts" ? (
          <ContactsSidebarPanelContent />
        ) : null}

        {active_primary_tab === "capabilities" ? (
          <div className="flex min-h-0 flex-1 flex-col px-2" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.capabilities_list}>
            <CapabilitiesPanelContent />
          </div>
        ) : null}
      </div>

      <div className="relative flex items-center justify-between gap-2.5 border-t divider-subtle px-3 py-3">
          <div className="flex items-center gap-2.5">
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

            <button
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                is_guide_center_open && "bg-(--surface-interactive-active-background) text-(--text-strong)",
              )}
              data-tour-anchor={SIDEBAR_TOUR_ANCHORS.restart}
              onClick={open_guide_center}
              title={t("common.guide_center")}
              type="button"
            >
              <Compass className="h-4 w-4" />
            </button>
          </div>

          <div className="min-w-0 flex-1" />

          <div className="flex items-center gap-2.5">
            {should_show_logout ? (
              <button
                aria-label={t("sidebar.logout")}
                className="flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
                onClick={() => {
                  void logout();
                }}
                title={t("sidebar.logout")}
                type="button"
              >
                <LogOut className="h-4 w-4" />
              </button>
            ) : null}

            <button
              aria-label={t("sidebar.collapse_panel")}
              className="flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              onClick={() => set_wide_panel_collapsed(true)}
              title={t("sidebar.collapse_panel")}
              type="button"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </div>
      </div>

      <OnboardingGuideCenter
        {...guide_center_props}
      />
    </div>
  );
}
