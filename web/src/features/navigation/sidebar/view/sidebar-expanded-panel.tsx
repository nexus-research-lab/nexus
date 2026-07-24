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
  SidebarHeaderActions,
} from "./sidebar-utility-actions";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./sidebar-wide-panel-types";

interface SidebarExpandedPanelProps {
  activeTab: SidebarPrimaryTab;
  dockUtilities: boolean;
  launcherLabel: string;
  navigationLabel: string;
  onPointerDown: PointerEventHandler<HTMLDivElement>;
  onPointerLeave: PointerEventHandler<HTMLDivElement>;
  onPointerMove: PointerEventHandler<HTMLDivElement>;
  onPointerUp: PointerEventHandler<HTMLDivElement>;
  onSelectTab: (tab: SidebarPrimaryTab) => void;
  resizable: boolean;
  resizeHotzoneActive: boolean;
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
  width: number | string;
}

const PANEL_CONTENT: Record<SidebarPrimaryTab, ComponentType> = {
  capabilities: CapabilityPanel,
  chat: ChatSidebarPanelContent,
  contacts: ContactsSidebarPanelContent,
};

export function SidebarExpandedPanel({
  activeTab,
  dockUtilities,
  launcherLabel,
  navigationLabel,
  onPointerDown,
  onPointerLeave,
  onPointerMove,
  onPointerUp,
  onSelectTab,
  resizable,
  resizeHotzoneActive,
  rootRef,
  settingsNavigation,
  showSplitEdge,
  tabs,
  utility,
  width,
}: SidebarExpandedPanelProps) {
  const ActivePanelContent = PANEL_CONTENT[activeTab];
  return (
    <div
      className={cn(
        "desktop-rail relative flex h-full shrink-0 flex-col",
        HOME_SIDEBAR_PADDING_CLASS,
        resizable && resizeHotzoneActive && "cursor-col-resize",
      )}
      onPointerDown={resizable ? onPointerDown : undefined}
      onPointerLeave={resizable ? onPointerLeave : undefined}
      onPointerMove={resizable ? onPointerMove : undefined}
      onPointerUp={resizable ? onPointerUp : undefined}
      ref={resizable ? rootRef : undefined}
      data-shell-split-edge={showSplitEdge ? "true" : undefined}
      style={{ width }}
    >
      <div
        className={cn(
          "shell-region-header -mr-1.5 flex items-center gap-2 pl-3 pr-[18px]",
          WORKSPACE_HEADER_HEIGHT_CLASS,
          "max-lg:px-4",
        )}
      >
        <SidebarBrandLink label={launcherLabel} />
        <SidebarHeaderActions {...utility} variant="panel" />
      </div>
      {settingsNavigation ? (
        <div className="flex min-h-0 flex-1 flex-col">
          {settingsNavigation}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <nav
            aria-label={navigationLabel}
            className="shell-navigation-rail flex w-[60px] shrink-0 flex-col"
          >
            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto">
              <SidebarPrimaryTabs
                activeTab={activeTab}
                items={tabs}
                onSelect={onSelectTab}
                variant="dock"
              />
            </div>
            {dockUtilities ? (
              <SidebarFooterActions {...utility} variant="rail" />
            ) : null}
          </nav>
          <div className="soft-scrollbar scrollbar-stable-gutter flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto py-2.5">
            <ActivePanelContent />
          </div>
        </div>
      )}
      {dockUtilities ? null : (
        <SidebarFooterActions {...utility} variant="panel" />
      )}
    </div>
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
