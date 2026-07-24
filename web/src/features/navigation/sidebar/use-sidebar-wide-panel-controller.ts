"use client";

import { useCallback, useEffect, useMemo } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import {
  getDefaultAgentAvatar,
  getDefaultAgentId,
  isMainAgent,
} from "@/config/runtime-options";
import { useChatCompletionNotifications } from "@/features/home/notifications/use-chat-completion-notifications";
import { useGuideCenterController } from "@/features/onboarding/guide-center/use-guide-center-controller";
import { resolveDirectRoomNavigationTarget } from "@/features/navigation/direct-room/direct-room-navigation";
import { getIconAvatarSrc } from "@/lib/avatar";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useAgentStore } from "@/store/agent";
import {
  SIDEBAR_CAPABILITY_ITEM_IDS,
  deriveSidebarItemIdFromPath,
  SIDEBAR_SYSTEM_ITEM_IDS,
  useSidebarStore,
} from "@/store/sidebar";

import {
  buildSidebarPrimaryTabs,
  buildSidebarUtilityLabels,
  deriveSidebarPrimaryTab,
  isNexusSidebarItemActive,
} from "./sidebar-wide-panel-model";
import type { SidebarPrimaryTab } from "./view/sidebar-wide-panel-types";
import { useSidebarPanelResize } from "./use-sidebar-panel-resize";

export function useSidebarWidePanelController({
  navigationOnly = false,
}: {
  navigationOnly?: boolean;
} = {}) {
  const { t } = useI18n();
  const { logout, status: authStatus } = useAuth();
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const agents = useAgentStore((state) => state.agents);
  const activePanelItemId = useSidebarStore((state) => state.active_panel_item_id);
  const chatBadgeCount = useSidebarStore((state) => state.chat_badge_count);
  const nexusRoomId = useSidebarStore((state) => state.nexus_room_id);
  const setActivePanelItem = useSidebarStore((state) => state.set_active_panel_item);
  const setWidePanelCollapsed = useSidebarStore(
    (state) => state.set_wide_panel_collapsed,
  );
  const setWidePanelWidth = useSidebarStore((state) => state.set_wide_panel_width);
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const widePanelCollapseSource = useSidebarStore(
    (state) => state.wide_panel_collapse_source,
  );
  const widePanelWidth = useSidebarStore((state) => state.wide_panel_width);
  const activeTab = deriveSidebarPrimaryTab(pathname);
  const defaultAgentId = getDefaultAgentId();
  const desktopRuntime = isDesktopRuntime();
  const settingsMode = pathname === AppRouteBuilders.settings();
  const nexusAgent = agents.find((agent) => isMainAgent(agent.agent_id)) ?? null;
  const nexusAvatar = nexusAgent?.avatar?.trim() || getDefaultAgentAvatar();
  const nexusActive = isNexusSidebarItemActive(
    activePanelItemId,
    nexusRoomId,
    SIDEBAR_SYSTEM_ITEM_IDS.nexus,
  );
  const selectedPrimaryTab = nexusActive ? null : activeTab;

  useChatCompletionNotifications();
  const guideCenter = useGuideCenterController({
    defaultAgentId,
    setActivePanelItem,
  });
  const resize = useSidebarPanelResize({
    setWidth: setWidePanelWidth,
    width: widePanelWidth,
  });

  useEffect(() => {
    const nextActiveItemId = deriveSidebarItemIdFromPath(pathname);
    if (nextActiveItemId !== activePanelItemId) {
      setActivePanelItem(nextActiveItemId);
    }
  }, [activePanelItemId, pathname, setActivePanelItem]);

  const openNexus = useCallback(() => {
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

  const selectPrimaryTab = useCallback((tab: SidebarPrimaryTab) => {
    const actions: Record<SidebarPrimaryTab, () => void> = {
      capabilities: () => {
        setActivePanelItem(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
        navigate(navigationOnly
          ? AppRouteBuilders.capability()
          : AppRouteBuilders.skills());
      },
      chat: () => {
        if (selectedPrimaryTab !== "chat") {
          navigate(AppRouteBuilders.home());
        }
      },
      contacts: () => {
        setActivePanelItem(null);
        navigate(AppRouteBuilders.contacts());
      },
    };
    actions[tab]();
  }, [navigate, navigationOnly, selectedPrimaryTab, setActivePanelItem]);

  const tabs = useMemo(
    () => buildSidebarPrimaryTabs(t, selectedPrimaryTab, chatBadgeCount),
    [chatBadgeCount, selectedPrimaryTab, t],
  );
  const utilityLabels = useMemo(() => buildSidebarUtilityLabels(t), [t]);

  return {
    collapseSource: widePanelCollapseSource,
    collapsed: widePanelCollapsed,
    expanded: {
      launcherLabel: t("sidebar.back_to_launcher"),
      onPointerDown: resize.handlePointerDown,
      onPointerLeave: resize.handlePointerLeave,
      onPointerMove: resize.handlePointerMove,
      onPointerUp: resize.handlePointerUp,
      resizeHotzoneActive: resize.isResizeHotzoneActive,
      rootRef: resize.rootRef,
      width: widePanelWidth,
    },
    guideCenterProps: guideCenter.guideCenterProps,
    settingsMode,
    shared: {
      activeTab,
      selectedPrimaryTab,
      nexus: {
        active: nexusActive,
        avatarSrc: getIconAvatarSrc(nexusAvatar),
        label: t("sidebar.workspace_title"),
        onClick: openNexus,
      },
      onSelectTab: selectPrimaryTab,
      tabs,
      utility: {
        guideOpen: guideCenter.isGuideCenterOpen,
        labels: utilityLabels,
        onCollapse: () => setWidePanelCollapsed(true),
        onExpand: () => setWidePanelCollapsed(false),
        onLogout: () => void logout(),
        onOpenGuide: guideCenter.openGuideCenter,
        settingsActive: pathname.startsWith(AppRouteBuilders.settings()),
        showLogout:
          !desktopRuntime
          && authStatus?.auth_required === true
          && authStatus.password_login_enabled
          && authStatus.authenticated,
        showPanelToggle: !navigationOnly,
        showSettings: !settingsMode,
      },
    },
  };
}
