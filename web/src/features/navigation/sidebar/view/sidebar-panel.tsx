/**
 * INPUT: 上层已装配的一级导航、固定会话、目录内容与侧栏系统动作。
 * OUTPUT: 展开/收起共用的主侧栏壳层及唯一导航轨。
 * POS: 主侧栏纯视图边界，不读取路由、Store 或业务 API。
 */
import type {
  ComponentType,
  PointerEventHandler,
  ReactNode,
  RefObject,
} from "react";

import { CapabilitySidebarPanel } from "@/features/capability/sidebar/capability-sidebar-panel";
import { ChatSidebarPanelContent } from "@/features/home/sidebar/chat-sidebar-panel";
import { ContactsSidebarPanelContent } from "@/features/home/sidebar/contacts-sidebar-panel";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

import { SidebarBrandLink } from "./sidebar-brand-link";
import { SidebarPinnedConversations } from "./sidebar-pinned-conversations";
import { SidebarPrimaryTabs } from "./sidebar-primary-tabs";
import {
  SidebarFooterActions,
  SidebarPanelToggleAction,
} from "./sidebar-utility-actions";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarPinnedConversationItem,
  SidebarPinnedConversationPlacement,
  SidebarUtilityLabels,
} from "./sidebar-wide-panel-types";

interface SidebarPanelProps {
  activeTab: SidebarPrimaryTab;
  collapsed: boolean;
  expandedWidth: number | string;
  launcherLabel: string;
  navigationLabel: string;
  onPointerDown: PointerEventHandler<HTMLDivElement>;
  onPointerLeave: PointerEventHandler<HTMLDivElement>;
  onPointerMove: PointerEventHandler<HTMLDivElement>;
  onPointerUp: PointerEventHandler<HTMLDivElement>;
  onSelectTab: (tab: SidebarPrimaryTab) => void;
  pinnedConversations: {
    items: SidebarPinnedConversationItem[];
    label: string;
    onReorder: (
      source: SidebarPinnedConversationItem,
      target: SidebarPinnedConversationItem,
      placement: SidebarPinnedConversationPlacement,
    ) => void;
    onSelect: (item: SidebarPinnedConversationItem) => void;
    onUnpin: (item: SidebarPinnedConversationItem) => void;
    reorderLabel: string;
    unpinLabel: string;
  };
  resizable: boolean;
  resizeHotzoneActive: boolean;
  resizing: boolean;
  rootRef: RefObject<HTMLDivElement | null>;
  settingsNavigation?: ReactNode;
  showSplitEdge: boolean;
  tabs: SidebarPrimaryTabItem[];
  utility: {
    guideOpen: boolean;
    labels: SidebarUtilityLabels;
    onCollapse: () => void;
    onExpand: () => void;
    onLogout: () => void;
    onOpenGuide: () => void;
    settingsActive: boolean;
    showLogout: boolean;
    showPanelToggle: boolean;
    showSettings: boolean;
  };
}

const PANEL_CONTENT: Record<SidebarPrimaryTab, ComponentType> = {
  capabilities: CapabilityPanel,
  chat: ChatSidebarPanelContent,
  contacts: ContactsSidebarPanelContent,
};

export function SidebarPanel({
  activeTab,
  collapsed,
  expandedWidth,
  launcherLabel,
  navigationLabel,
  onPointerDown,
  onPointerLeave,
  onPointerMove,
  onPointerUp,
  onSelectTab,
  pinnedConversations,
  resizable,
  resizeHotzoneActive,
  resizing,
  rootRef,
  settingsNavigation,
  showSplitEdge,
  tabs,
  utility,
}: SidebarPanelProps) {
  const ActivePanelContent = PANEL_CONTENT[activeTab];
  const width = collapsed ? 0 : expandedWidth;
  const resizeEnabled = resizable && !collapsed;

  return (
    <>
      {collapsed && utility.showPanelToggle ? (
        <div className="sidebar-panel-collapsed-toggle">
          <SidebarPanelToggleAction
            labels={utility.labels}
            onCollapse={utility.onCollapse}
            onExpand={utility.onExpand}
            showPanelToggle
            variant="rail"
          />
        </div>
      ) : null}
      <div
        aria-hidden={collapsed || undefined}
        className={cn(
          "sidebar-panel-shell desktop-rail relative flex h-full shrink-0 flex-col overflow-hidden",
          !collapsed && HOME_SIDEBAR_PADDING_CLASS,
          resizeEnabled && resizeHotzoneActive && "cursor-col-resize",
        )}
        data-shell-split-edge={showSplitEdge && !collapsed ? "true" : undefined}
        data-sidebar-collapsed={collapsed ? "true" : undefined}
        data-sidebar-resizing={
          resizeEnabled && resizing ? "true" : undefined
        }
        onLostPointerCapture={resizeEnabled ? onPointerUp : undefined}
        onPointerCancel={resizeEnabled ? onPointerUp : undefined}
        onPointerDown={resizeEnabled ? onPointerDown : undefined}
        onPointerLeave={resizeEnabled ? onPointerLeave : undefined}
        onPointerMove={resizeEnabled ? onPointerMove : undefined}
        onPointerUp={resizeEnabled ? onPointerUp : undefined}
        inert={collapsed ? true : undefined}
        ref={resizeEnabled ? rootRef : undefined}
        style={{ width }}
      >
        <div
          className={cn(
            "sidebar-panel-header shell-region-header -mr-1.5 flex shrink-0 items-center",
            WORKSPACE_HEADER_HEIGHT_CLASS,
            collapsed ? "px-2" : "pl-3 pr-[18px]",
            "max-lg:px-4",
          )}
          data-desktop-window-controls-leading={
            collapsed ? undefined : "true"
          }
          data-desktop-window-drag-region
        >
          <SidebarBrandLink collapsed={collapsed} label={launcherLabel} />
          <div aria-hidden="true" className="min-w-0 flex-1 self-stretch" />
          {!collapsed ? (
            <div className="sidebar-panel-header-toggle shrink-0">
              <SidebarPanelToggleAction
                labels={utility.labels}
                onCollapse={utility.onCollapse}
                onExpand={utility.onExpand}
                showPanelToggle={utility.showPanelToggle}
                variant="panel"
              />
            </div>
          ) : null}
        </div>
        {settingsNavigation ? (
          <div
            className={cn(
              "flex min-h-0 flex-1 flex-col",
              collapsed && "-mr-1.5",
            )}
          >
            {settingsNavigation}
          </div>
        ) : (
          <div className="flex min-h-0 flex-1">
            <nav
              aria-label={navigationLabel}
              className="shell-navigation-rail flex min-h-0 w-16 shrink-0 flex-col"
            >
              <div className="w-full shrink-0">
                <SidebarPrimaryTabs
                  activeTab={activeTab}
                  items={tabs}
                  onSelect={onSelectTab}
                />
              </div>
              <SidebarPinnedConversations {...pinnedConversations} />
            </nav>
            <div
              className={cn(
                "sidebar-panel-directory soft-scrollbar scrollbar-stable-gutter flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto py-2.5",
                collapsed && "pointer-events-none opacity-0",
              )}
            >
              <ActivePanelContent />
            </div>
          </div>
        )}
        {collapsed ? null : <SidebarFooterActions {...utility} />}
      </div>
    </>
  );
}

function CapabilityPanel() {
  return (
    <div
      className="flex min-h-0 flex-1 flex-col"
      data-tour-anchor={SIDEBAR_TOUR_ANCHORS.capabilities_list}
    >
      <CapabilitySidebarPanel />
    </div>
  );
}
