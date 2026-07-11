import type { ReactNode } from "react";

import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/lib/utils";

import { SidebarNexusButton } from "./sidebar-nexus-button";
import { SidebarPrimaryTabs } from "./sidebar-primary-tabs";
import { SidebarUtilityActions } from "./sidebar-utility-actions";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./sidebar-wide-panel-types";

interface SidebarCollapsedRailProps {
  activeTab: SidebarPrimaryTab;
  nexus: {
    active: boolean;
    avatarSrc: string | null;
    onClick: () => void;
    prefersReducedMotion: boolean;
  };
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
    showSettings: boolean;
  };
}

export function SidebarCollapsedRail({
  activeTab,
  nexus,
  onSelectTab,
  settingsNavigation,
  tabs,
  utility,
}: SidebarCollapsedRailProps) {
  return (
    <aside
      className={cn(
        "desktop-rail relative flex h-full w-[56px] shrink-0 flex-col items-center",
        HOME_SIDEBAR_PADDING_CLASS,
      )}
      data-sidebar-collapsed="true"
    >
      <div className="flex min-h-0 flex-1 flex-col items-center gap-2 pb-3 pt-2">
        <SidebarNexusButton {...nexus} variant="rail" />
        {settingsNavigation ?? (
          <SidebarPrimaryTabs
            activeTab={activeTab}
            items={tabs}
            onSelect={onSelectTab}
            variant="rail"
          />
        )}
      </div>
      <SidebarUtilityActions {...utility} variant="rail" />
    </aside>
  );
}
