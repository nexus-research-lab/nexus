"use client";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";

export const LAUNCHER_TOUR_ID = "launcher-guide";

export const LAUNCHER_TOUR_ANCHORS = {
  enter_app: "launcher-enter-app",
  composer: "launcher-composer",
  recent: "launcher-recent",
  handoff: "launcher-handoff",
} as const;

export function build_launcher_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: LAUNCHER_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("launcher.tour_intro_title"),
        description: t("launcher.tour_intro_description"),
        placement: "center",
        image: "/nexus/welcome.png",
      },
      {
        id: "composer",
        title: t("launcher.tour_composer_title"),
        description: t("launcher.tour_composer_description"),
        target: LAUNCHER_TOUR_ANCHORS.composer,
        placement: "bottom",
        image: "/nexus/writing.png",
      },
      {
        id: "recent",
        title: t("launcher.tour_recent_title"),
        description: t("launcher.tour_recent_description"),
        target: LAUNCHER_TOUR_ANCHORS.recent,
        placement: "bottom",
        image: "/nexus/reading.png",
      },
      {
        id: "handoff",
        title: t("launcher.tour_handoff_title"),
        description: t("launcher.tour_handoff_description"),
        target: LAUNCHER_TOUR_ANCHORS.handoff,
        placement: "bottom",
        image: "/nexus/assigning.png",
      },
      {
        id: "enter_app",
        title: t("launcher.tour_enter_app_title"),
        description: t("launcher.tour_enter_app_description"),
        target: LAUNCHER_TOUR_ANCHORS.enter_app,
        placement: "bottom",
        image: "/nexus/pointing.png",
      },
    ],
  };
}
