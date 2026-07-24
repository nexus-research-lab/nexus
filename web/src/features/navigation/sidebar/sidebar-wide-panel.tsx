import { GuideCenterDialog } from "@/features/onboarding/guide-center/guide-center-dialog";
import { SettingsSidebarNavigation } from "@/features/settings/settings-sidebar-navigation";

import { SidebarCollapsedRail } from "./view/sidebar-collapsed-rail";
import { SidebarExpandedPanel } from "./view/sidebar-expanded-panel";
import { useSidebarWidePanelController } from "./use-sidebar-wide-panel-controller";

interface SidebarWidePanelProps {
  fillAvailableWidth?: boolean;
  navigationOnly?: boolean;
}

export function SidebarWidePanel({
  fillAvailableWidth = false,
  navigationOnly = false,
}: SidebarWidePanelProps) {
  const controller = useSidebarWidePanelController({ navigationOnly });
  const nexusFocusMode =
    !fillAvailableWidth &&
    controller.collapseSource !== "manual" &&
    controller.shared.nexus.active;
  const showCollapsedRail =
    !fillAvailableWidth && (controller.collapsed || nexusFocusMode);
  const settingsNavigation = controller.settingsMode
    ? (
        <SettingsSidebarNavigation
          variant={showCollapsedRail ? "rail" : "panel"}
        />
      )
    : undefined;

  return (
    <>
      {showCollapsedRail ? (
        <SidebarCollapsedRail
          {...controller.shared}
          launcherLabel={controller.expanded.launcherLabel}
          mode={nexusFocusMode ? "focus" : "collapsed"}
          settingsNavigation={settingsNavigation}
        />
      ) : (
        <SidebarExpandedPanel
          {...controller.shared}
          {...controller.expanded}
          dockUtilities={fillAvailableWidth}
          resizable={!fillAvailableWidth}
          resizeHotzoneActive={
            fillAvailableWidth
              ? false
              : controller.expanded.resizeHotzoneActive
          }
          settingsNavigation={settingsNavigation}
          showSplitEdge={!fillAvailableWidth}
          width={fillAvailableWidth ? "100%" : controller.expanded.width}
        />
      )}
      <GuideCenterDialog {...controller.guideCenterProps} />
    </>
  );
}
