"use client";

import { useCallback } from "react";
import { useNavigate, useSearchParams } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import {
  isDesktopBridgeAvailable,
  openDesktopRoute,
} from "@/lib/desktop-bridge";

import {
  parseSettingsSection,
  type SettingsSectionKey,
} from "./settings-navigation-model";

export function useSettingsNavigation() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const activeSection = parseSettingsSection(searchParams);

  const selectSection = useCallback(
    (section: SettingsSectionKey) => {
      navigate(AppRouteBuilders.settings(section));
    },
    [navigate],
  );

  const backToWorkspace = useCallback(() => {
    if (isDesktopBridgeAvailable()) {
      void openDesktopRoute(AppRouteBuilders.home()).catch((error) => {
        console.error("[SettingsNavigation] 桌面返回工作台失败:", error);
        navigate(AppRouteBuilders.home());
      });
      return;
    }
    navigate(AppRouteBuilders.home());
  }, [navigate]);

  return {
    activeSection,
    backToWorkspace,
    selectSection,
  };
}
