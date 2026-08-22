/**
 * INPUT: 当前常规设置分区与偏好控制器状态。
 * OUTPUT: 单一分区标题及对应设置内容，移动端由应用栏承载页面身份。
 * POS: 常规、外观、工作区和权限分区的页面装配层。
 */
"use client";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { WorkspaceContentHeader } from "@/shared/ui/layout/workspace-content-header";
import { WORKSPACE_CONTENT_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";

import { SettingsAppearanceSection } from "./sections/settings-appearance-section";
import { SettingsDesktopSection } from "./sections/settings-desktop-section";
import { SettingsGeneralBehaviorSection } from "./sections/settings-general-behavior-section";
import { SettingsPermissionsSection } from "./sections/settings-permissions-section";
import { SettingsWorkspaceSection } from "./sections/settings-workspace-section";
import { useGeneralSettingsController } from "./use-general-settings-controller";

type GeneralSettingsSectionKey =
  | "general"
  | "appearance"
  | "workspace"
  | "permissions";

const SETTINGS_SECTION_COPY: Record<
  GeneralSettingsSectionKey,
  { description: TranslationKey; title: TranslationKey }
> = {
  appearance: {
    description: "settings.general.section_appearance_description",
    title: "settings.general.section_appearance",
  },
  general: {
    description: "settings.general.section_general_description",
    title: "settings.general.section_general",
  },
  permissions: {
    description: "settings.general.section_permissions_description",
    title: "settings.general.section_permissions",
  },
  workspace: {
    description: "settings.general.section_workspace_description",
    title: "settings.general.section_workspace",
  },
};

export function SettingsGeneralSection({
  section,
}: {
  section: GeneralSettingsSectionKey;
}) {
  const { t } = useI18n();
  const copy = SETTINGS_SECTION_COPY[section];

  return (
    <div
      className={cn(
        WORKSPACE_CONTENT_PAGE_CLASS_NAME,
        "flex flex-col",
      )}
    >
      <WorkspaceContentHeader
        className="max-sm:hidden"
        description={t(copy.description)}
        title={t(copy.title)}
      />
      <div className="flex flex-col gap-5">
        {section === "general" ? (
          <>
            <SettingsDesktopSection />
            <SettingsGeneralBehaviorContent />
          </>
        ) : null}
        {section === "appearance" ? <SettingsAppearanceSection /> : null}
        {section === "workspace" ? <SettingsWorkspaceSection /> : null}
        {section === "permissions" ? (
          <SettingsPermissionsContent />
        ) : null}
      </div>
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
