"use client";

import { cn } from "@/lib/utils";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";

import { SettingsAppearanceSection } from "../settings-appearance-section";
import { SettingsDesktopSection } from "../settings-desktop-section";
import { SettingsSystemSection } from "../settings-system-section";
import { SettingsGeneralBehaviorSection } from "./sections/settings-general-behavior-section";
import { SettingsPermissionsSection } from "./sections/settings-permissions-section";
import { SettingsWorkspaceSection } from "./sections/settings-workspace-section";
import { useGeneralSettingsController } from "./use-general-settings-controller";

export function SettingsGeneralSection() {
  const model = useGeneralSettingsController();

  return (
    <div
      className={cn(
        "mx-auto flex w-full flex-col gap-5 px-1 py-3",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}
    >
      <SettingsSystemSection />
      <SettingsAppearanceSection />
      <SettingsGeneralBehaviorSection {...model.behavior} />
      <SettingsWorkspaceSection />
      <SettingsDesktopSection />
      <SettingsPermissionsSection {...model.permissions} />
    </div>
  );
}
