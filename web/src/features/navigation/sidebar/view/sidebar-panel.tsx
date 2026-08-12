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
import { SidebarPrimaryTabs } from "./sidebar-primary-tabs";
import {
  SidebarFooterActions,
  SidebarPanelToggleAction,
} from "./sidebar-utility-actions";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
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

const COLLAPSED_SIDEBAR_WIDTH = 52;

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
  const width = collapsed ? COLLAPSED_SIDEBAR_WIDTH : expandedWidth;
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
        className={cn(
          "sidebar-panel-shell desktop-rail relative flex h-full shrink-0 flex-col overflow-hidden",
          HOME_SIDEBAR_PADDING_CLASS,
          resizeEnabled && resizeHotzoneActive && "cursor-col-resize",
        )}
        data-shell-split-edge={showSplitEdge ? "true" : undefined}
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
              className="shell-navigation-rail flex w-12 shrink-0 flex-col"
            >
              <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto">
                <SidebarPrimaryTabs
                  activeTab={activeTab}
                  items={tabs}
                  onSelect={onSelectTab}
                />
              </div>
            </nav>
            <div
              aria-hidden={collapsed || undefined}
              className={cn(
                "sidebar-panel-directory soft-scrollbar scrollbar-stable-gutter flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto py-2.5",
                collapsed && "pointer-events-none opacity-0",
              )}
              inert={collapsed ? true : undefined}
            >
              <ActivePanelContent />
            </div>
          </div>
        )}
        <SidebarFooterActions {...utility} collapsed={collapsed} />
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
