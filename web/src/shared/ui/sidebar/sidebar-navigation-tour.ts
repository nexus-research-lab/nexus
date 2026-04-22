"use client";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";

export const SIDEBAR_NAVIGATION_TOUR_ID = "sidebar-navigation";

export const SIDEBAR_TOUR_ANCHORS = {
  agents: "sidebar-agents-section",
  rooms: "sidebar-rooms-section",
  capabilities: "sidebar-capabilities-section",
} as const;

export function build_sidebar_navigation_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: SIDEBAR_NAVIGATION_TOUR_ID,
    steps: [
      {
        id: "agents",
        title: t("sidebar.tour_agents_title"),
        description: t("sidebar.tour_agents_description"),
        target: SIDEBAR_TOUR_ANCHORS.agents,
        placement: "right",
      },
      {
        id: "rooms",
        title: t("sidebar.tour_rooms_title"),
        description: t("sidebar.tour_rooms_description"),
        target: SIDEBAR_TOUR_ANCHORS.rooms,
        placement: "right",
      },
      {
        id: "capabilities",
        title: t("sidebar.tour_capabilities_title"),
        description: t("sidebar.tour_capabilities_description"),
        target: SIDEBAR_TOUR_ANCHORS.capabilities,
        placement: "right",
      },
    ],
  };
}
