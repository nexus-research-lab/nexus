"use client";

import { cn } from "@/lib/utils";
import { WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";

import { SettingsAppearanceSection } from "./sections/settings-appearance-section";
import { SettingsDesktopSection } from "./sections/settings-desktop-section";
import { SettingsSystemSection } from "./sections/settings-system-section";
import { SettingsGeneralBehaviorSection } from "./sections/settings-general-behavior-section";
import { SettingsPermissionsSection } from "./sections/settings-permissions-section";
import { SettingsWorkspaceSection } from "./sections/settings-workspace-section";
import { useGeneralSettingsController } from "./use-general-settings-controller";

type GeneralSettingsSectionKey =
  | "general"
  | "appearance"
  | "workspace"
  | "permissions";

export function SettingsGeneralSection({
  section,
}: {
  section: GeneralSettingsSectionKey;
}) {
  return (
    <div
      className={cn(
        "mx-auto flex w-full flex-col gap-5 px-1 py-3",
        WORKSPACE_DETAIL_MAX_WIDTH_CLASS_NAME,
      )}
    >
      {section === "general" ? (
        <>
          <SettingsSystemSection />
          <SettingsGeneralBehaviorContent />
        </>
      ) : null}
      {section === "appearance" ? <SettingsAppearanceSection /> : null}
      {section === "workspace" ? (
        <>
          <SettingsWorkspaceSection />
          <SettingsDesktopSection />
        </>
      ) : null}
      {section === "permissions" ? (
        <SettingsPermissionsContent />
      ) : null}
    </div>
  );
}

function SettingsGeneralBehaviorContent() {
  const { behavior } = useGeneralSettingsController();
  return <SettingsGeneralBehaviorSection {...behavior} />;
}

function SettingsPermissionsContent() {
  const { permissions } = useGeneralSettingsController();
  return <SettingsPermissionsSection {...permissions} />;
}
