import { GuideCenterDialog } from "@/features/onboarding/guide-center/guide-center-dialog";
import { SettingsSidebarNavigation } from "@/features/settings/settings-sidebar-navigation";

import { SidebarPanel } from "./view/sidebar-panel";
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
  const collapsed =
    !fillAvailableWidth && controller.collapsed;
  const settingsNavigation = controller.settingsMode
    ? (
        <SettingsSidebarNavigation
          variant={collapsed ? "rail" : "panel"}
        />
      )
    : undefined;

  return (
    <>
      <SidebarPanel
        {...controller.shared}
        {...controller.expanded}
        collapsed={collapsed}
        expandedWidth={fillAvailableWidth ? "100%" : controller.expanded.width}
        resizable={!fillAvailableWidth}
        resizeHotzoneActive={
          fillAvailableWidth
            ? false
            : controller.expanded.resizeHotzoneActive
        }
        resizing={
          fillAvailableWidth
            ? false
            : controller.expanded.resizing
        }
        settingsNavigation={settingsNavigation}
        showSplitEdge={!fillAvailableWidth}
      />
      <GuideCenterDialog {...controller.guideCenterProps} />
    </>
  );
}
