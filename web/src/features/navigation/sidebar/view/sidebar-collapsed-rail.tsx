import type { ReactNode } from "react";

import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

import { SidebarBrandLink } from "./sidebar-brand-link";
import { SidebarNexusButton } from "./sidebar-nexus-button";
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
  launcherLabel: string;
  mode: "collapsed" | "focus";
  nexus: {
    active: boolean;
    avatarSrc: string | null;
    label: string;
    onClick: () => void;
  };
  onSelectTab: (tab: SidebarPrimaryTab) => void;
  selectedPrimaryTab: SidebarPrimaryTab | null;
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
  launcherLabel,
  mode,
  nexus,
  onSelectTab,
  selectedPrimaryTab,
  settingsNavigation,
  tabs,
  utility,
}: SidebarCollapsedRailProps) {
  const focus = mode === "focus";
  return (
    <aside
      className={cn(
        "desktop-rail shell-navigation-rail relative flex h-full shrink-0 flex-col items-center",
        focus ? "w-[224px]" : "w-[56px]",
        HOME_SIDEBAR_PADDING_CLASS,
      )}
      data-shell-split-edge="true"
      data-sidebar-collapsed={focus ? undefined : "true"}
      data-sidebar-focus={focus ? "true" : undefined}
    >
      <div
        className={cn(
          "shell-region-header -mr-1.5 flex shrink-0 self-stretch items-center",
          focus
            ? cn(WORKSPACE_HEADER_HEIGHT_CLASS, "gap-1 pl-2 pr-[14px]")
            : cn(WORKSPACE_HEADER_HEIGHT_CLASS, "justify-center"),
        )}
      >
        {focus ? (
          <>
            <SidebarBrandLink label={launcherLabel} variant="focus" />
            <SidebarHeaderActions {...utility} variant="panel" />
          </>
        ) : (
          <SidebarHeaderActions
            {...utility}
            showLogout={false}
            variant="rail"
          />
        )}
      </div>
      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col items-center overflow-y-auto pb-3 pt-1">
        {settingsNavigation ?? (
          <nav aria-label={nexus.label}>
            <SidebarPrimaryTabs
              activeTab={selectedPrimaryTab}
              items={tabs}
              leading={
                <SidebarNexusButton
                  {...nexus}
                  variant={focus ? "focus" : "rail"}
                />
              }
              onSelect={onSelectTab}
              variant={focus ? "focus" : "rail"}
            />
          </nav>
        )}
      </div>
      <div className="-mr-1.5 self-stretch">
        <SidebarFooterActions
          {...utility}
          variant={focus ? "panel" : "rail"}
        />
      </div>
    </aside>
  );
}
