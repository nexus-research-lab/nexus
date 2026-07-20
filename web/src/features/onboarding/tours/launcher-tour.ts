import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-contract";

export const LAUNCHER_TOUR_ID = "launcher-guide";

export const LAUNCHER_TOUR_ANCHORS = {
  enter_app: "launcher-enter-app",
  composer: "launcher-composer",
  recent: "launcher-recent",
  handoff: "launcher-handoff",
} as const;

export function buildLauncherTour(
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
        image: "/nexus/stickers/guide-launcher.png",
      },
      {
        id: "composer",
        title: t("launcher.tour_composer_title"),
        description: t("launcher.tour_composer_description"),
        target: LAUNCHER_TOUR_ANCHORS.composer,
        placement: "bottom",
      },
      {
        id: "recent",
        title: t("launcher.tour_recent_title"),
        description: t("launcher.tour_recent_description"),
        target: LAUNCHER_TOUR_ANCHORS.recent,
        placement: "bottom",
      },
      {
        id: "handoff",
        title: t("launcher.tour_handoff_title"),
        description: t("launcher.tour_handoff_description"),
        target: LAUNCHER_TOUR_ANCHORS.handoff,
        placement: "bottom",
      },
      {
        id: "enter_app",
        title: t("launcher.tour_enter_app_title"),
        description: t("launcher.tour_enter_app_description"),
        target: LAUNCHER_TOUR_ANCHORS.enter_app,
        placement: "bottom",
      },
    ],
  };
}
