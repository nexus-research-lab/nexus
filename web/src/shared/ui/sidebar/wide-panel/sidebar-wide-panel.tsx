import { MessageCircle, Puzzle, Users2 } from "lucide-react";
import { useCallback, useEffect } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import {
  getDefaultAgentAvatar,
  getDefaultAgentId,
  isMainAgent,
} from "@/config/options";
import { useChatCompletionNotifications } from "@/features/home/notifications/use-chat-completion-notifications";
import { SettingsSidebarNavigation } from "@/features/settings/settings-sidebar-navigation";
import { usePrefersReducedMotion } from "@/hooks/ui/use-prefers-reduced-motion";
import { resolveDirectRoomNavigationTarget } from "@/lib/conversation/direct-room-navigation";
import { getIconAvatarSrc } from "@/lib/utils";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { OnboardingGuideCenter } from "@/shared/ui/onboarding/onboarding-guide-center";
import { SIDEBAR_TOUR_ANCHORS } from "@/shared/ui/sidebar/sidebar-navigation-tour";
import { useSidebarGuideCenter } from "@/shared/ui/sidebar/use-sidebar-guide-center";
import { useSidebarPanelResize } from "@/shared/ui/sidebar/use-sidebar-panel-resize";
import { useAgentStore } from "@/store/agent";
import {
  SIDEBAR_CAPABILITY_ITEM_IDS,
  deriveSidebarItemIdFromPath,
  SIDEBAR_SYSTEM_ITEM_IDS,
  useSidebarStore,
} from "@/store/sidebar";

import { SidebarCollapsedRail } from "./sidebar-collapsed-rail";
import { SidebarExpandedPanel } from "./sidebar-expanded-panel";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./sidebar-wide-panel-types";

export function SidebarWidePanel() {
  const { t } = useI18n();
  const { logout } = useAuth();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const agents = useAgentStore((state) => state.agents);
  const activePanelItemId = useSidebarStore(
    (state) => state.active_panel_item_id,
  );
  const nexusRoomId = useSidebarStore((state) => state.nexus_room_id);
  const chatBadgeCount = useSidebarStore((state) => state.chat_badge_count);
  const setActivePanelItem = useSidebarStore(
    (state) => state.set_active_panel_item,
  );
  const widePanelWidth = useSidebarStore((state) => state.wide_panel_width);
  const setWidePanelWidth = useSidebarStore(
    (state) => state.set_wide_panel_width,
  );
  const widePanelCollapsed = useSidebarStore(
    (state) => state.wide_panel_collapsed,
  );
  const setWidePanelCollapsed = useSidebarStore(
    (state) => state.set_wide_panel_collapsed,
  );
  const desktopRuntime = isDesktopRuntime();
  const settingsMode = pathname === AppRouteBuilders.settings();
  const activePrimaryTab = derivePrimaryTabFromPath(pathname);
  const defaultAgentId = getDefaultAgentId();
  const nexusAgent = agents.find((agent) => isMainAgent(agent.agent_id)) ?? null;
  const nexusAvatar = nexusAgent?.avatar?.trim() || getDefaultAgentAvatar();
  const isNexusActive =
    activePanelItemId === SIDEBAR_SYSTEM_ITEM_IDS.nexus ||
    (nexusRoomId ? activePanelItemId === nexusRoomId : false);
  const prefersReducedMotion = usePrefersReducedMotion();

  useChatCompletionNotifications();
  const guide = useSidebarGuideCenter({
    default_agent_id: defaultAgentId,
    set_active_panel_item: setActivePanelItem,
  });
  const resize = useSidebarPanelResize({
    set_wide_panel_width: setWidePanelWidth,
    wide_panel_width: widePanelWidth,
  });

  useEffect(() => {
    const nextActiveItemId = deriveSidebarItemIdFromPath(pathname);
    if (nextActiveItemId !== activePanelItemId) {
      setActivePanelItem(nextActiveItemId);
    }
  }, [activePanelItemId, pathname, setActivePanelItem]);

  const handleOpenNexus = useCallback(() => {
    if (!defaultAgentId) {
      return;
    }
    setActivePanelItem(SIDEBAR_SYSTEM_ITEM_IDS.nexus);
    void resolveDirectRoomNavigationTarget(defaultAgentId)
      .then(({ route }) => navigate(route))
      .catch((error) => {
        console.error("[SidebarWidePanel] 打开 Nexus DM 失败:", error);
      });
  }, [defaultAgentId, navigate, setActivePanelItem]);
  const handleSelectPrimaryTab = useCallback(
    (tab: SidebarPrimaryTab) => {
      const actions: Record<SidebarPrimaryTab, () => void> = {
        capabilities: () => {
          setActivePanelItem(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
          navigate(AppRouteBuilders.skills());
        },
        chat: () => {
          if (!pathname.startsWith("/rooms/")) {
            navigate(AppRouteBuilders.home());
          }
        },
        contacts: () => {
          setActivePanelItem(null);
          navigate(AppRouteBuilders.contacts());
        },
      };
      actions[tab]();
    },
    [navigate, pathname, setActivePanelItem],
  );

  const tabs: SidebarPrimaryTabItem[] = [
    {
      anchor: SIDEBAR_TOUR_ANCHORS.chat_tab,
      badgeCount: activePrimaryTab === "chat" ? 0 : chatBadgeCount,
      icon: MessageCircle,
      key: "chat",
      label: t("sidebar.tab_chat"),
    },
    {
      anchor: SIDEBAR_TOUR_ANCHORS.contacts_tab,
      badgeCount: 0,
      icon: Users2,
      key: "contacts",
      label: t("sidebar.tab_contacts"),
    },
    {
      anchor: SIDEBAR_TOUR_ANCHORS.capabilities_tab,
      badgeCount: 0,
      icon: Puzzle,
      key: "capabilities",
      label: t("sidebar.tab_capabilities"),
    },
  ];
  const utilityLabels: SidebarUtilityLabels = {
    collapse: t("sidebar.collapse_panel"),
    expand: t("sidebar.expand_panel"),
    guide: t("common.guide_center"),
    logout: t("sidebar.logout"),
    settings: t("sidebar.settings"),
  };
  const nexus = {
    active: isNexusActive,
    avatarSrc: getIconAvatarSrc(nexusAvatar),
    onClick: handleOpenNexus,
    prefersReducedMotion,
  };
  const utility = {
    guideOpen: guide.is_guide_center_open,
    labels: utilityLabels,
    onCollapse: () => setWidePanelCollapsed(true),
    onExpand: () => setWidePanelCollapsed(false),
    onLogout: () => void logout(),
    onOpenGuide: guide.open_guide_center,
    settingsActive: pathname.startsWith(AppRouteBuilders.settings()),
    showLogout: !desktopRuntime,
    showSettings: !settingsMode,
  };
  const sharedProps = {
    activeTab: activePrimaryTab,
    nexus,
    onSelectTab: handleSelectPrimaryTab,
    tabs,
    utility,
  };

  return (
    <>
      {widePanelCollapsed ? (
        <SidebarCollapsedRail
          {...sharedProps}
          settingsNavigation={
            settingsMode ? <SettingsSidebarNavigation variant="rail" /> : undefined
          }
        />
      ) : (
        <SidebarExpandedPanel
          {...sharedProps}
          launcherLabel={t("sidebar.back_to_launcher")}
          onPointerDown={resize.handle_pointer_down}
          onPointerLeave={resize.handle_pointer_leave}
          onPointerMove={resize.handle_pointer_move}
          onPointerUp={resize.handle_pointer_up}
          resizeHotzoneActive={resize.is_resize_hotzone_active}
          rootRef={resize.root_ref}
          settingsNavigation={
            settingsMode ? <SettingsSidebarNavigation variant="panel" /> : undefined
          }
          width={widePanelWidth}
        />
      )}
      <OnboardingGuideCenter {...guide.guide_center_props} />
    </>
  );
}

function derivePrimaryTabFromPath(pathname: string): SidebarPrimaryTab {
  if (pathname.startsWith(AppRouteBuilders.contacts())) {
    return "contacts";
  }
  if (pathname.startsWith("/capability/")) {
    return "capabilities";
  }
  return "chat";
}
