import type { ReactNode } from "react";

import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

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

interface SidebarCollapsedRailProps {
  activeTab: SidebarPrimaryTab;
  navigationLabel: string;
  onSelectTab: (tab: SidebarPrimaryTab) => void;
  settingsNavigation?: ReactNode;
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

export function SidebarCollapsedRail({
  activeTab,
  navigationLabel,
  onSelectTab,
  settingsNavigation,
  tabs,
  utility,
}: SidebarCollapsedRailProps) {
  return (
    <aside
      className={cn(
        "desktop-rail shell-navigation-rail relative flex h-full w-[56px] shrink-0 flex-col items-center",
        HOME_SIDEBAR_PADDING_CLASS,
      )}
      data-shell-split-edge="true"
      data-sidebar-collapsed="true"
    >
      <div
        className={cn(
          "shell-region-header -mr-1.5 flex shrink-0 items-center justify-center self-stretch",
          WORKSPACE_HEADER_HEIGHT_CLASS,
        )}
      >
        <SidebarHeaderActions
          {...utility}
          showLogout={false}
          variant="rail"
        />
      </div>
      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col items-center overflow-y-auto pb-3 pt-1">
        {settingsNavigation ?? (
          <nav aria-label={navigationLabel}>
            <SidebarPrimaryTabs
              activeTab={activeTab}
              items={tabs}
              onSelect={onSelectTab}
              variant="rail"
            />
          </nav>
        )}
      </div>
      <div className="-mr-1.5 self-stretch">
        <SidebarFooterActions
          {...utility}
          variant="rail"
        />
      </div>
    </aside>
  );
}
