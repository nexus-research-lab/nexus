"use client";

import {
  ArrowLeft,
  Cable,
  Palette,
  UserRound,
} from "lucide-react";
import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import {
  isDesktopBridgeAvailable,
  openDesktopRoute,
} from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { PersonalSettingsPanel } from "./personal-settings-panel";
import { ProviderSettingsPanel } from "./provider-settings-panel";
import { SettingsGeneralSection } from "./settings-general-section";

type SettingsTabKey =
  | "general"
  | "personal"
  | "providers";

const SETTINGS_TABS: {
  key: SettingsTabKey;
  labelKey:
    | "settings.tabs.general"
    | "settings.tabs.personal"
    | "settings.tabs.providers";
  icon: typeof Palette;
}[] = [
  { key: "general", labelKey: "settings.tabs.general", icon: Palette },
  { key: "personal", labelKey: "settings.tabs.personal", icon: UserRound },
  { key: "providers", labelKey: "settings.tabs.providers", icon: Cable },
];

export function SettingsPanel() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [activeTab, setActiveTab] = useState<SettingsTabKey>("general");
  const activeTabConfig =
    SETTINGS_TABS.find((item) => item.key === activeTab) ?? SETTINGS_TABS[0];
  const ActiveIcon = activeTabConfig.icon;

  const handleBackToWorkspace = useCallback(() => {
    if (isDesktopBridgeAvailable()) {
      void openDesktopRoute(APP_ROUTE_PATHS.home).catch((error) => {
        console.error("[SettingsPanel] 桌面返回工作台失败:", error);
        navigate(APP_ROUTE_PATHS.home);
      });
      return;
    }
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

  return (
    <WorkspaceSurfaceScaffold
      bodyScrollable
      stableGutter
      header={(
        <WorkspaceSurfaceHeader
          activeTab={activeTab}
          density="compact"
          leading={<ActiveIcon className="h-4 w-4" />}
          onChangeTab={setActiveTab}
          tabs={SETTINGS_TABS.map((item) => ({
            key: item.key,
            label: t(item.labelKey),
            icon: item.icon,
          }))}
          title={t("settings.title")}
          trailing={(
            <WorkspaceSurfaceToolbarAction onClick={handleBackToWorkspace}>
              <ArrowLeft className="h-3.5 w-3.5" />
              {t("settings.back_to_workspace")}
            </WorkspaceSurfaceToolbarAction>
          )}
        />
      )}
    >
      {activeTab === "general" ? <SettingsGeneralSection /> : null}
      {activeTab === "personal" ? <PersonalSettingsPanel /> : null}
      {activeTab === "providers" ? <ProviderSettingsPanel embedded /> : null}
    </WorkspaceSurfaceScaffold>
  );
}
