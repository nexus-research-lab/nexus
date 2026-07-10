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
  ShieldCheck,
  type LucideIcon,
  Users2,
} from "lucide-react";
import { useCallback, useEffect } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getDefaultAgentAvatar, getDefaultAgentId, isMainAgent } from "@/config/options";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { CapabilitiesPanelContent } from "@/features/capability/capabilities-sidebar-panel";
import {
  ChatSidebarPanelContent,
  ContactsSidebarPanelContent,
} from "@/features/home/home-sidebar-panel";
import { useChatCompletionNotifications } from "@/features/home/use-chat-completion-notifications";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { resolveDirectRoomNavigationTarget } from "@/lib/conversation/direct-room-navigation";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn, getIconAvatarSrc } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiCounterBadge } from "@/shared/ui/badge";
import { OnboardingGuideCenter } from "@/shared/ui/onboarding/onboarding-guide-center";
import { COMPACT_WORKSPACE_HEADER_SINGLE_ROW_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import {
  SIDEBAR_TOUR_ANCHORS,
} from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useSidebarGuideCenter } from "@/shared/ui/sidebar/use-sidebar-guide-center";
import { useSidebarPanelResize } from "@/shared/ui/sidebar/use-sidebar-panel-resize";

import { GlassMagnifierStatic } from "@/shared/ui/liquid-glass";
import { useAgentStore } from "@/store/agent";
import {
  SIDEBAR_CAPABILITY_ITEM_IDS,
  deriveSidebarItemIdFromPath,
  SIDEBAR_SYSTEM_ITEM_IDS,
  useSidebarStore,
} from "@/store/sidebar";

type SidebarPrimaryTab = "chat" | "contacts" | "capabilities";

function derivePrimaryTabFromPath(pathname: string): SidebarPrimaryTab {
  if (pathname.startsWith(AppRouteBuilders.contacts())) {
    return "contacts";
  }
  if (pathname.startsWith("/capability/")) {
    return "capabilities";
  }
  return "chat";
}

export function SidebarWidePanel() {
  const { t } = useI18n();
  const { logout, status } = useAuth();
  const location = useLocation();
  const { pathname } = location;
  const navigate = useNavigate();
  const agents = useAgentStore((s) => s.agents);
  const activePanelItemId = useSidebarStore((s) => s.active_panel_item_id);
  const nexusRoomId = useSidebarStore((s) => s.nexus_room_id);
  const chatBadgeCount = useSidebarStore((s) => s.chat_badge_count);
  const setActivePanelItem = useSidebarStore((s) => s.set_active_panel_item);
  const widePanelWidth = useSidebarStore((s) => s.wide_panel_width);
  const setWidePanelWidth = useSidebarStore((s) => s.set_wide_panel_width);
  const widePanelCollapsed = useSidebarStore((s) => s.wide_panel_collapsed);
  const setWidePanelCollapsed = useSidebarStore((s) => s.set_wide_panel_collapsed);
  const toggleWidePanelCollapsed = useSidebarStore((s) => s.toggle_wide_panel_collapsed);
  const isSettingsRoute = pathname.startsWith(AppRouteBuilders.settings());
  const isOperationsRoute = pathname.startsWith(AppRouteBuilders.operations());
  const activePrimaryTab = derivePrimaryTabFromPath(pathname);
  const shouldShowLogout = !isDesktopRuntime();
  const canViewOperations =
    !isDesktopRuntime() && (status?.role === "owner" || status?.role === "admin");
  const prefersReducedMotion = usePrefersReducedMotion();
  const defaultAgentId = getDefaultAgentId();
  const nexusAgent = agents.find((agent) => isMainAgent(agent.agent_id)) ?? null;
  const nexusAvatar = nexusAgent?.avatar?.trim() || getDefaultAgentAvatar();
  const nexusAvatarSrc = getIconAvatarSrc(nexusAvatar);
  const isNexusActive = activePanelItemId === SIDEBAR_SYSTEM_ITEM_IDS.nexus
    || (nexusRoomId ? activePanelItemId === nexusRoomId : false);
  useChatCompletionNotifications();
  const {
    guide_center_props: guideCenterProps,
    is_guide_center_open: isGuideCenterOpen,
    open_guide_center: openGuideCenter,
  } = useSidebarGuideCenter({
    default_agent_id: defaultAgentId,
    set_active_panel_item: setActivePanelItem,
  });

  const {
    handle_pointer_down: handlePointerDown,
    handle_pointer_leave: handlePointerLeave,
    handle_pointer_move: handlePointerMove,
    handle_pointer_up: handlePointerUp,
    is_resize_hotzone_active: isResizeHotzoneActive,
    root_ref: rootRef,
  } = useSidebarPanelResize({
    set_wide_panel_width: setWidePanelWidth,
    wide_panel_width: widePanelWidth,
  });

  /** 路由变化时统一同步侧栏高亮，避免能力和房间走两套状态。 */
  useEffect(() => {
    const nextActiveItemId = deriveSidebarItemIdFromPath(pathname);
    if (nextActiveItemId === activePanelItemId) {
      return;
    }
    setActivePanelItem(nextActiveItemId);
  }, [activePanelItemId, pathname, setActivePanelItem]);

  const handleOpenNexus = useCallback(() => {
    if (!defaultAgentId) {
      return;
    }

    setActivePanelItem(SIDEBAR_SYSTEM_ITEM_IDS.nexus);
    void resolveDirectRoomNavigationTarget(defaultAgentId).then(({ route }) => {
      navigate(route);
    }).catch((error) => {
      console.error("[SidebarWidePanel] 打开 Nexus DM 失败:", error);
    });
  }, [defaultAgentId, navigate, setActivePanelItem]);

  const handleSelectPrimaryTab = useCallback((tab: SidebarPrimaryTab) => {
    if (tab === "chat") {
      if (!pathname.startsWith("/rooms/")) {
        navigate(AppRouteBuilders.home());
      }
      return;
    }

    if (tab === "contacts") {
      setActivePanelItem(null);
      navigate(AppRouteBuilders.contacts());
      return;
    }

    setActivePanelItem(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
    navigate(AppRouteBuilders.skills());
  }, [navigate, pathname, setActivePanelItem]);

  const primaryTabs: {
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
      badge_count: activePrimaryTab === "chat" ? 0 : chatBadgeCount,
    },
    { key: "contacts", label: t("sidebar.tab_contacts"), icon: Users2, anchor: SIDEBAR_TOUR_ANCHORS.contacts_tab },
    { key: "capabilities", label: t("sidebar.tab_capabilities"), icon: Puzzle, anchor: SIDEBAR_TOUR_ANCHORS.capabilities_tab },
  ];

  if (widePanelCollapsed) {
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
              isNexusActive && "border-(--surface-interactive-active-border) shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_10%,transparent)]",
            )}
            onClick={handleOpenNexus}
            title="Nexus"
            type="button"
          >
            {nexusAvatarSrc ? (
              <img
                alt=""
                className="h-full w-full object-cover"
                src={nexusAvatarSrc}
              />
            ) : "NX"}
          </button>

          <div className="mt-1 flex flex-col items-center gap-1.5">
            {primaryTabs.map((tab) => {
              const Icon = tab.icon;
              const isActive = activePrimaryTab === tab.key;
              return (
                <button
                  aria-label={tab.label}
                  aria-current={isActive ? "page" : undefined}
                  className={cn(
                    "relative flex h-9 w-9 items-center justify-center rounded-full text-(--icon-default) transition-(background,color,transform) duration-(--motion-duration-fast) hover:-translate-y-px hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                    isActive && "bg-(--surface-interactive-active-background) text-(--primary)",
                  )}
                  key={tab.key}
                  onClick={() => handleSelectPrimaryTab(tab.key)}
                  title={tab.label}
                  type="button"
                >
                  <Icon
                    className={cn(
                      "h-4 w-4",
                      isActive && "fill-(--primary) stroke-(--primary)",
                    )}
                  />
                  <UiCounterBadge
                    className="absolute -right-1 -top-1 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
                    count={tab.badge_count ?? 0}
                  />
                </button>
              );
            })}
          </div>
        </div>

        <div className="flex flex-col items-center gap-1.5 border-t divider-subtle py-3">
          {canViewOperations ? (
            <Link
              aria-label={t("sidebar.operations")}
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                isOperationsRoute && "bg-(--surface-interactive-active-background) text-(--text-strong)",
              )}
              title={t("sidebar.operations")}
              to={AppRouteBuilders.operations()}
            >
              <ShieldCheck className="h-4 w-4" />
            </Link>
          ) : null}

          <Link
            aria-label={t("sidebar.settings")}
            className={cn(
              "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
              isSettingsRoute && "bg-(--surface-interactive-active-background) text-(--text-strong)",
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
              isGuideCenterOpen && "bg-(--surface-interactive-active-background) text-(--text-strong)",
            )}
            onClick={openGuideCenter}
            title={t("common.guide_center")}
            type="button"
          >
            <Compass className="h-4 w-4" />
          </button>

          {shouldShowLogout ? (
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
            onClick={toggleWidePanelCollapsed}
            title={t("sidebar.expand_panel")}
            type="button"
          >
            <PanelLeftOpen className="h-4 w-4" />
          </button>
        </div>

        <OnboardingGuideCenter
          {...guideCenterProps}
        />
      </aside>
    );
  }

  return (
    <div
      className={cn(
        "desktop-rail relative flex h-full shrink-0 flex-col",
        HOME_SIDEBAR_PADDING_CLASS,
        isResizeHotzoneActive && "cursor-col-resize",
      )}
      onPointerDown={handlePointerDown}
      onPointerLeave={handlePointerLeave}
      onPointerMove={handlePointerMove}
      onPointerUp={handlePointerUp}
      ref={rootRef}
      style={{ width: widePanelWidth }}
    >
      {/* 面板头部 */}
      <div className={cn(
        "grid grid-cols-[46px_minmax(0,1fr)] items-center gap-1.5 border-b divider-subtle px-3",
        COMPACT_WORKSPACE_HEADER_SINGLE_ROW_HEIGHT_CLASS,
      )}>
        <button
          className="group/nexus relative flex h-10 w-[46px] shrink-0 items-center justify-center"
          data-tour-anchor={SIDEBAR_TOUR_ANCHORS.nexus_agent}
          onClick={handleOpenNexus}
          title="Nexus"
          type="button"
        >
          <GlassMagnifierStatic
            className={cn(
              "relative z-10 transition-transform duration-(--motion-duration-normal)",
              !prefersReducedMotion && "group-hover/nexus:scale-[1.03]",
              isNexusActive && "drop-shadow-[0_8px_20px_color-mix(in_srgb,var(--primary)_12%,transparent)]",
            )}
            height={34}
            underlay={isNexusActive ? (
              <>
                {/* 中文注释：把圆形彩光作为玻璃组件的下层内容，保证折射和高光都基于真实下层，而不是页面层假叠加。 */}
                <span
                  className={cn(
                    "absolute left-1/2 top-1/2 h-[36px] w-[36px] -translate-x-1/2 -translate-y-1/2 rounded-full opacity-88 blur-[0.5px]",
                    !prefersReducedMotion && "animate-[spin_5.2s_linear_infinite]",
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
                    !prefersReducedMotion && "animate-[spin_8.6s_linear_infinite_reverse]",
                  )}
                  style={{
                    background: "conic-gradient(from 180deg, transparent 0deg, rgba(96,165,250,0.84) 66deg, transparent 136deg, transparent 214deg, rgba(244,114,182,0.82) 292deg, rgba(52,211,153,0.74) 336deg, transparent 360deg)",
                  }}
                />
                <span className="absolute left-1/2 top-1/2 h-[24px] w-[24px] -translate-x-1/2 -translate-y-1/2 rounded-full bg-[radial-gradient(circle_at_34%_28%,rgba(255,255,255,0.34),transparent_42%),radial-gradient(circle_at_68%_72%,rgba(255,255,255,0.14),transparent_48%)] opacity-82 blur-[3px]" />
              </>
            ) : undefined}
            width={46}
          >
            <span className="relative flex h-7 w-7 items-center justify-center">
              <span
                className={cn(
                  "relative z-10 flex h-7 w-7 items-center justify-center overflow-hidden rounded-full border border-(--surface-avatar-border) bg-(--surface-avatar-background) shadow-(--surface-avatar-shadow)",
                  isNexusActive && "shadow-[0_0_0_1px_rgba(255,255,255,0.14),0_0_10px_color-mix(in_srgb,var(--primary)_8%,transparent)]",
                )}
              >
                {isNexusActive ? (
                  <>
                    {/* 中文注释：这一层只做很轻的玻璃反光，不再承担主动画；主动态来自下层彩光被玻璃折射。 */}
                    <span className="pointer-events-none absolute inset-0 z-20 rounded-full bg-[radial-gradient(circle_at_28%_24%,rgba(255,255,255,0.24),transparent_38%),linear-gradient(132deg,rgba(255,255,255,0.18),transparent_42%,transparent_60%,rgba(255,255,255,0.08))] mix-blend-screen opacity-72" />
                    <span className="pointer-events-none absolute inset-[1px] z-20 rounded-full border border-[rgba(255,255,255,0.22)] opacity-72" />
                  </>
                ) : null}
                {nexusAvatarSrc ? (
                  <img
                    alt="Nexus"
                    className="relative z-10 h-full w-full object-cover"
                    src={nexusAvatarSrc}
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
              className="whitespace-nowrap text-[18px] uppercase tracking-[0.07em] text-(--text-default)"
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
          {primaryTabs.map((tab) => {
            const Icon = tab.icon;
            const isActive = activePrimaryTab === tab.key;
            return (
              <button
                aria-current={isActive ? "page" : undefined}
                aria-pressed={isActive}
                className={cn(
                  "flex h-9 items-center justify-center gap-1.5 rounded-[11px] text-[13px] font-medium transition-[background,color,box-shadow] duration-(--motion-duration-fast)",
                  isActive
                    ? "bg-[color:color-mix(in_srgb,var(--primary)_14%,var(--surface-elevated-background))] text-(--primary) shadow-[0_8px_22px_color-mix(in_srgb,var(--primary)_10%,transparent)]"
                    : "text-(--text-muted) hover:text-(--text-strong)",
                )}
                data-tour-anchor={tab.anchor}
                key={tab.key}
                onClick={() => handleSelectPrimaryTab(tab.key)}
                type="button"
              >
                <span className="relative flex h-4 w-4 items-center justify-center">
                  <Icon
                    className={cn(
                      "h-3.5 w-3.5",
                      isActive && "fill-(--primary) stroke-(--primary)",
                    )}
                  />
                  <UiCounterBadge
                    className="absolute -right-2.5 -top-2 h-4 min-w-4 px-1 text-[10px] shadow-[0_2px_6px_rgba(255,76,84,0.28)]"
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
        {activePrimaryTab === "chat" ? (
          <ChatSidebarPanelContent />
        ) : null}

        {activePrimaryTab === "contacts" ? (
          <ContactsSidebarPanelContent />
        ) : null}

        {activePrimaryTab === "capabilities" ? (
          <div className="flex min-h-0 flex-1 flex-col px-2" data-tour-anchor={SIDEBAR_TOUR_ANCHORS.capabilities_list}>
            <CapabilitiesPanelContent />
          </div>
        ) : null}
      </div>

      <div className="relative flex items-center justify-between gap-2.5 border-t divider-subtle px-3 py-3">
          <div className="flex items-center gap-2.5">
            {canViewOperations ? (
              <Link
                className={cn(
                  "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                  isOperationsRoute && "bg-(--surface-interactive-active-background) text-(--text-strong)",
                )}
                title={t("sidebar.operations")}
                to={AppRouteBuilders.operations()}
              >
                <ShieldCheck className="h-4 w-4" />
              </Link>
            ) : null}

            <Link
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                isSettingsRoute && "bg-(--surface-interactive-active-background) text-(--text-strong)",
              )}
              title={t("sidebar.settings")}
              to={AppRouteBuilders.settings()}
            >
              <Settings className="h-4 w-4" />
            </Link>

            <button
              className={cn(
                "flex h-8 w-8 items-center justify-center rounded-full text-(--icon-default) transition-(background,color) duration-(--motion-duration-normal) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
                isGuideCenterOpen && "bg-(--surface-interactive-active-background) text-(--text-strong)",
              )}
              data-tour-anchor={SIDEBAR_TOUR_ANCHORS.restart}
              onClick={openGuideCenter}
              title={t("common.guide_center")}
              type="button"
            >
              <Compass className="h-4 w-4" />
            </button>
          </div>

          <div className="min-w-0 flex-1" />

          <div className="flex items-center gap-2.5">
            {shouldShowLogout ? (
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
              onClick={() => setWidePanelCollapsed(true)}
              title={t("sidebar.collapse_panel")}
              type="button"
            >
              <PanelLeftClose className="h-4 w-4" />
            </button>
          </div>
      </div>

      <OnboardingGuideCenter
        {...guideCenterProps}
      />
    </div>
  );
}
