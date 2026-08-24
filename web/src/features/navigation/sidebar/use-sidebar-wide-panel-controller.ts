/**
 * INPUT: 当前路由、鉴权/i18n 状态、侧栏 Store 与面板拖拽动作。
 * OUTPUT: 宽侧栏导航、进入聊天时的入口红点确认、折叠和退出行为模型。
 * POS: 纯侧栏控制层；全局聊天完成订阅由 AppLayout 持有，不能下沉到此处。
 */
"use client";

import { useCallback, useEffect, useMemo } from "react";
import { useLocation, useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { isDesktopRuntime } from "@/config/desktop-runtime";
import { getDefaultAgentId } from "@/config/runtime-options";
import { useHomeDirectory } from "@/features/home/home-directory-resource";
import { useGuideCenterController } from "@/features/onboarding/guide-center/use-guide-center-controller";
import { useAuth } from "@/shared/auth/auth-context";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  SIDEBAR_CAPABILITY_ITEM_IDS,
  deriveSidebarItemIdFromPath,
  useSidebarStore,
} from "@/store/sidebar";
import { useRoomNavigationStore } from "@/store/room-navigation";

import {
  buildSidebarPinnedConversations,
  buildSidebarPrimaryTabs,
  buildSidebarUtilityLabels,
  deriveSidebarPrimaryTab,
} from "./sidebar-wide-panel-model";
import type { SidebarPrimaryTab } from "./view/sidebar-wide-panel-types";
import type { SidebarPinnedConversationItem } from "./view/sidebar-wide-panel-types";
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
  const directory = useHomeDirectory();
  const pinnedConversations = useRoomNavigationStore(
    (state) => state.pinned_conversations,
  );
  const unpinConversation = useRoomNavigationStore(
    (state) => state.unpin_conversation,
  );
  const activePanelItemId = useSidebarStore((state) => state.active_panel_item_id);
  const chatBadgeCount = useSidebarStore((state) => state.chat_badge_count);
  const acknowledgeChatTab = useSidebarStore(
    (state) => state.acknowledge_chat_tab,
  );
  const setActivePanelItem = useSidebarStore((state) => state.set_active_panel_item);
  const setWidePanelCollapsed = useSidebarStore(
    (state) => state.set_wide_panel_collapsed,
  );
  const setWidePanelWidth = useSidebarStore((state) => state.set_wide_panel_width);
  const widePanelCollapsed = useSidebarStore((state) => state.wide_panel_collapsed);
  const widePanelWidth = useSidebarStore((state) => state.wide_panel_width);
  const activeTab = deriveSidebarPrimaryTab(pathname);
  const defaultAgentId = getDefaultAgentId();
  const desktopRuntime = isDesktopRuntime();
  const settingsMode = pathname === AppRouteBuilders.settings();

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

  useEffect(() => {
    if (activeTab === "chat") {
      acknowledgeChatTab();
    }
  }, [acknowledgeChatTab, activeTab]);

  const selectPrimaryTab = useCallback((tab: SidebarPrimaryTab) => {
    const actions: Record<SidebarPrimaryTab, () => void> = {
      capabilities: () => {
        setActivePanelItem(SIDEBAR_CAPABILITY_ITEM_IDS.skills);
        navigate(navigationOnly
          ? AppRouteBuilders.capability()
          : AppRouteBuilders.skills());
      },
      chat: () => {
        if (activeTab !== "chat") {
          navigate(AppRouteBuilders.home());
        }
      },
      contacts: () => {
        setActivePanelItem(null);
        navigate(AppRouteBuilders.contacts());
      },
    };
    actions[tab]();
  }, [activeTab, navigate, navigationOnly, setActivePanelItem]);

  const tabs = useMemo(
    () => buildSidebarPrimaryTabs(t, activeTab, chatBadgeCount),
    [activeTab, chatBadgeCount, t],
  );
  const pinnedConversationItems = useMemo(
    () => buildSidebarPinnedConversations({
      conversations: directory.conversations,
      pathname,
      pinnedConversations,
      untitledLabel: t("room.new_conversation"),
    }),
    [directory.conversations, pathname, pinnedConversations, t],
  );
  const utilityLabels = useMemo(() => buildSidebarUtilityLabels(t), [t]);
  const selectPinnedConversation = useCallback((item: SidebarPinnedConversationItem) => {
    navigate(item.route);
  }, [navigate]);
  const removePinnedConversation = useCallback((item: SidebarPinnedConversationItem) => {
    unpinConversation(item.roomId, item.conversationId);
  }, [unpinConversation]);

  return {
    collapsed: widePanelCollapsed,
    expanded: {
      launcherLabel: t("sidebar.back_to_launcher"),
      onPointerDown: resize.handlePointerDown,
      onPointerLeave: resize.handlePointerLeave,
      onPointerMove: resize.handlePointerMove,
      onPointerUp: resize.handlePointerUp,
      resizeHotzoneActive: resize.isResizeHotzoneActive,
      resizing: resize.isResizing,
      rootRef: resize.rootRef,
      width: widePanelWidth,
    },
    guideCenterProps: guideCenter.guideCenterProps,
    settingsMode,
    shared: {
      activeTab,
      navigationLabel: t("sidebar.workspace_title"),
      onSelectTab: selectPrimaryTab,
      pinnedConversations: {
        items: pinnedConversationItems,
        label: t("sidebar.pinned_conversations"),
        onSelect: selectPinnedConversation,
        onUnpin: removePinnedConversation,
        unpinLabel: t("sidebar.unpin_conversation"),
      },
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
