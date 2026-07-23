import type {
  ComponentType,
  PointerEventHandler,
  ReactNode,
  RefObject,
} from "react";
import { Link } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { CapabilitySidebarPanel } from "@/features/capability/sidebar/capability-sidebar-panel";
import { ChatSidebarPanelContent } from "@/features/home/sidebar/chat-sidebar-panel";
import { ContactsSidebarPanelContent } from "@/features/home/sidebar/contacts-sidebar-panel";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import { HOME_SIDEBAR_PADDING_CLASS } from "@/lib/layout/home-layout";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_HEADER_HEIGHT_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";

import { SidebarNexusButton } from "./sidebar-nexus-button";
import { SidebarPrimaryTabs } from "./sidebar-primary-tabs";
import { SidebarUtilityActions } from "./sidebar-utility-actions";
import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./sidebar-wide-panel-types";

interface SidebarExpandedPanelProps {
  activeTab: SidebarPrimaryTab;
  dockUtilities: boolean;
  launcherLabel: string;
  nexus: {
    active: boolean;
    avatarSrc: string | null;
    onClick: () => void;
    prefersReducedMotion: boolean;
  };
  onPointerDown: PointerEventHandler<HTMLDivElement>;
  onPointerLeave: PointerEventHandler<HTMLDivElement>;
  onPointerMove: PointerEventHandler<HTMLDivElement>;
  onPointerUp: PointerEventHandler<HTMLDivElement>;
  onSelectTab: (tab: SidebarPrimaryTab) => void;
  resizeHotzoneActive: boolean;
  rootRef: RefObject<HTMLDivElement | null>;
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
  nexus,
  onPointerDown,
  onPointerLeave,
  onPointerMove,
  onPointerUp,
  onSelectTab,
  resizeHotzoneActive,
  rootRef,
  settingsNavigation,
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
        resizeHotzoneActive && "cursor-col-resize",
      )}
      onPointerDown={onPointerDown}
      onPointerLeave={onPointerLeave}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      ref={rootRef}
      style={{ width }}
    >
      <div
        className={cn(
          "grid grid-cols-[46px_minmax(0,1fr)] items-center gap-1.5 border-b divider-subtle px-3",
          WORKSPACE_HEADER_HEIGHT_CLASS,
          "max-lg:px-4",
        )}
      >
        <SidebarNexusButton {...nexus} variant="panel" />
        <Link
          className="block min-w-0"
          data-tour-anchor={SIDEBAR_TOUR_ANCHORS.launcher}
          title={launcherLabel}
          to={AppRouteBuilders.launcher()}
        >
          <p
            className="relative isolate whitespace-nowrap px-4 text-[22px] uppercase leading-none tracking-[0.42em]"
            style={{
              fontFamily: '"Panchang", var(--font-sans)',
              fontWeight: 420,
            }}
          >
            <span
              aria-hidden="true"
              className="absolute inset-x-4 top-0 translate-y-[1.5px] text-[color:color-mix(in_srgb,var(--text-strong)_38%,transparent)] opacity-60 blur-[0.2px]"
            >
              NEXUS
            </span>
            <span
              className="relative bg-clip-text text-transparent"
              style={{
                backgroundImage:
                  "linear-gradient(180deg, color-mix(in srgb, var(--text-strong) 94%, white 6%) 4%, var(--text-default) 48%, color-mix(in srgb, var(--text-muted) 72%, var(--text-strong) 28%) 100%)",
                filter:
                  "drop-shadow(0 1px 0 color-mix(in srgb, white 38%, transparent)) drop-shadow(0 4px 6px color-mix(in srgb, var(--text-strong) 12%, transparent))",
                WebkitBackgroundClip: "text",
              }}
            >
              NEXUS
            </span>
          </p>
        </Link>
      </div>
      {settingsNavigation ? (
        <div className="flex min-h-0 flex-1 flex-col">
          {settingsNavigation}
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <nav className="flex w-[60px] shrink-0 flex-col border-r divider-subtle">
            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto">
              <SidebarPrimaryTabs
                activeTab={activeTab}
                items={tabs}
                onSelect={onSelectTab}
                variant="dock"
              />
            </div>
            {dockUtilities ? (
              <SidebarUtilityActions {...utility} variant="rail" />
            ) : null}
          </nav>
          <div className="soft-scrollbar scrollbar-stable-gutter flex min-h-0 min-w-0 flex-1 flex-col overflow-y-auto py-2.5">
            <ActivePanelContent />
          </div>
        </div>
      )}
      {dockUtilities ? null : (
        <SidebarUtilityActions {...utility} variant="panel" />
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
