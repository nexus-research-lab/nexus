"use client";

import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { OnboardingTourDefinition } from "@/shared/ui/onboarding/tour-provider";

export const SKILLS_TOUR_ID = "skills-directory";

export const SKILLS_TOUR_ANCHORS = {
  catalog: "skills-catalog",
  categories: "skills-categories",
  header: "skills-header",
  import_git: "skills-import-git",
  import_local: "skills-import-local",
  modes: "skills-modes",
  search: "skills-search",
  update_library: "skills-update-library",
} as const;

export function build_skills_tour(
  t: I18nContextValue["t"],
): OnboardingTourDefinition {
  return {
    id: SKILLS_TOUR_ID,
    steps: [
      {
        id: "intro",
        title: t("capability.skills_tour_intro_title"),
        description: t("capability.skills_tour_intro_description"),
        image: "/nexus/working.png",
        placement: "center",
      },
      {
        id: "modes",
        title: t("capability.skills_tour_modes_title"),
        description: t("capability.skills_tour_modes_description"),
        target: SKILLS_TOUR_ANCHORS.modes,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "import-local",
        title: t("capability.skills_tour_import_local_title"),
        description: t("capability.skills_tour_import_local_description"),
        target: SKILLS_TOUR_ANCHORS.import_local,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "import-git",
        title: t("capability.skills_tour_import_git_title"),
        description: t("capability.skills_tour_import_git_description"),
        target: SKILLS_TOUR_ANCHORS.import_git,
        image: "/nexus/pointing.png",
        placement: "bottom",
      },
      {
        id: "update-library",
        title: t("capability.skills_tour_update_title"),
        description: t("capability.skills_tour_update_description"),
        target: SKILLS_TOUR_ANCHORS.update_library,
        image: "/nexus/working.png",
        placement: "bottom",
      },
      {
        id: "search",
        title: t("capability.skills_tour_search_title"),
        description: t("capability.skills_tour_search_description"),
        target: SKILLS_TOUR_ANCHORS.search,
        image: "/nexus/reading.png",
        placement: "bottom",
      },
      {
        id: "categories",
        title: t("capability.skills_tour_categories_title"),
        description: t("capability.skills_tour_categories_description"),
        target: SKILLS_TOUR_ANCHORS.categories,
        image: "/nexus/reviewing.png",
        placement: "bottom",
      },
      {
        id: "catalog",
        title: t("capability.skills_tour_catalog_title"),
        description: t("capability.skills_tour_catalog_description"),
        target: SKILLS_TOUR_ANCHORS.catalog,
        image: "/nexus/reviewing.png",
        placement: "top",
      },
    ],
  };
}
