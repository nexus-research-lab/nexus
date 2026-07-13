import { MessageCircle, Puzzle, Users2 } from "lucide-react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { SIDEBAR_TOUR_ANCHORS } from "@/features/onboarding/tours/sidebar-navigation-tour";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";

import type {
  SidebarPrimaryTab,
  SidebarPrimaryTabItem,
  SidebarUtilityLabels,
} from "./view/sidebar-wide-panel-types";

const PRIMARY_TAB_DEFINITIONS = [
  {
    anchor: SIDEBAR_TOUR_ANCHORS.chat_tab,
    icon: MessageCircle,
    key: "chat",
    labelKey: "sidebar.tab_chat",
  },
  {
    anchor: SIDEBAR_TOUR_ANCHORS.contacts_tab,
    icon: Users2,
    key: "contacts",
    labelKey: "sidebar.tab_contacts",
  },
  {
    anchor: SIDEBAR_TOUR_ANCHORS.capabilities_tab,
    icon: Puzzle,
    key: "capabilities",
    labelKey: "sidebar.tab_capabilities",
  },
] as const satisfies readonly {
  anchor: string;
  icon: SidebarPrimaryTabItem["icon"];
  key: SidebarPrimaryTab;
  labelKey: TranslationKey;
}[];

export function deriveSidebarPrimaryTab(pathname: string): SidebarPrimaryTab {
  if (pathname.startsWith(AppRouteBuilders.contacts())) {
    return "contacts";
  }
  if (pathname.startsWith("/capability/")) {
    return "capabilities";
  }
  return "chat";
}

export function buildSidebarPrimaryTabs(
  t: I18nContextValue["t"],
  activeTab: SidebarPrimaryTab,
  chatBadgeCount: number,
): SidebarPrimaryTabItem[] {
  return PRIMARY_TAB_DEFINITIONS.map((definition) => ({
    anchor: definition.anchor,
    badgeCount: definition.key === "chat" && activeTab !== "chat"
      ? chatBadgeCount
      : 0,
    icon: definition.icon,
    key: definition.key,
    label: t(definition.labelKey),
  }));
}

export function buildSidebarUtilityLabels(
  t: I18nContextValue["t"],
): SidebarUtilityLabels {
  return {
    collapse: t("sidebar.collapse_panel"),
    expand: t("sidebar.expand_panel"),
    guide: t("common.guide_center"),
    logout: t("sidebar.logout"),
    settings: t("sidebar.settings"),
  };
}

export function isNexusSidebarItemActive(
  activePanelItemId: string | null,
  nexusRoomId: string | null,
  nexusItemId: string,
): boolean {
  return activePanelItemId === nexusItemId
    || Boolean(nexusRoomId && activePanelItemId === nexusRoomId);
}
