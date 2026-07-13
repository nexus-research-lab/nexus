import { GuideCenterDialog } from "@/features/onboarding/guide-center/guide-center-dialog";
import { SettingsSidebarNavigation } from "@/features/settings/settings-sidebar-navigation";

import { SidebarCollapsedRail } from "./view/sidebar-collapsed-rail";
import { SidebarExpandedPanel } from "./view/sidebar-expanded-panel";
import { useSidebarWidePanelController } from "./use-sidebar-wide-panel-controller";

export function SidebarWidePanel() {
  const controller = useSidebarWidePanelController();
  const settingsNavigation = controller.settingsMode
    ? <SettingsSidebarNavigation variant={controller.collapsed ? "rail" : "panel"} />
    : undefined;

  return (
    <>
      {controller.collapsed ? (
        <SidebarCollapsedRail
          {...controller.shared}
          settingsNavigation={settingsNavigation}
        />
      ) : (
        <SidebarExpandedPanel
          {...controller.shared}
          {...controller.expanded}
          settingsNavigation={settingsNavigation}
        />
      )}
      <GuideCenterDialog {...controller.guideCenterProps} />
    </>
  );
}
