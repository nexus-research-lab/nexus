"use client";

import { ArrowLeft, Cable, Palette, UserRound } from "lucide-react";
import { useCallback, useState } from "react";
import { useNavigate } from "react-router-dom";

import { APP_ROUTE_PATHS } from "@/app/router/route-paths";
import {
  is_desktop_bridge_available,
  open_desktop_route,
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

type SettingsTabKey = "general" | "personal" | "providers";

const SETTINGS_TABS: {
  key: SettingsTabKey;
  label_key:
    | "settings.tabs.general"
    | "settings.tabs.personal"
    | "settings.tabs.providers";
  icon: typeof Palette;
}[] = [
  { key: "general", label_key: "settings.tabs.general", icon: Palette },
  { key: "personal", label_key: "settings.tabs.personal", icon: UserRound },
  { key: "providers", label_key: "settings.tabs.providers", icon: Cable },
];

export function SettingsPanel() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const [active_tab, set_active_tab] = useState<SettingsTabKey>("general");
  const active_tab_config =
    SETTINGS_TABS.find((item) => item.key === active_tab) ?? SETTINGS_TABS[0];
  const ActiveIcon = active_tab_config.icon;
  const handle_back_to_workspace = useCallback(() => {
    if (is_desktop_bridge_available()) {
      void open_desktop_route(APP_ROUTE_PATHS.home).catch((error) => {
        console.error("[SettingsPanel] 桌面返回工作台失败:", error);
        navigate(APP_ROUTE_PATHS.home);
      });
      return;
    }
    navigate(APP_ROUTE_PATHS.home);
  }, [navigate]);

  return (
    <WorkspaceSurfaceScaffold
      body_scrollable
      stable_gutter
      header={(
        <WorkspaceSurfaceHeader
          active_tab={active_tab}
          density="compact"
          leading={<ActiveIcon className="h-4 w-4" />}
          on_change_tab={set_active_tab}
          tabs={SETTINGS_TABS.map((item) => ({
            key: item.key,
            label: t(item.label_key),
            icon: item.icon,
          }))}
          title={t("settings.title")}
          trailing={(
            <WorkspaceSurfaceToolbarAction onClick={handle_back_to_workspace}>
              <ArrowLeft className="h-3.5 w-3.5" />
              {t("settings.back_to_workspace")}
            </WorkspaceSurfaceToolbarAction>
          )}
        />
      )}
    >
      {active_tab === "general" ? <SettingsGeneralSection /> : null}
      {active_tab === "personal" ? <PersonalSettingsPanel /> : null}
      {active_tab === "providers" ? <ProviderSettingsPanel embedded /> : null}
    </WorkspaceSurfaceScaffold>
  );
}
