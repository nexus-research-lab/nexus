import { SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import type { SkillMarketplaceController } from "./skills-view-model";
import { SKILLS_TOUR_ANCHORS } from "./skills-tour";

interface SkillsSearchBarProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsSearchBar({ ctrl }: SkillsSearchBarProps) {
  const { t } = useI18n();

  return (
    <div className="mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
      <CapabilityFilterSearchInput
        on_change={(value) => {
          if (ctrl.discovery_mode === "catalog") {
            ctrl.set_search_query(value);
            return;
          }
          ctrl.set_external_query(value);
        }}
        placeholder={
          ctrl.discovery_mode === "catalog"
            ? t("capability.skills_search_catalog")
            : t("capability.skills_search_external")
        }
        value={ctrl.discovery_mode === "catalog" ? ctrl.search_query : ctrl.external_query}
      />

      {ctrl.discovery_mode === "catalog" ? (
        <CapabilityFilterSelect
          aria_label={t("capability.skills_filter_aria")}
          label={t("capability.category_label")}
          leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
          on_change={ctrl.set_active_category}
          options={ctrl.categories.map((category) => ({
            label: category.label,
            value: category.key,
          }))}
          placeholder={t("capability.category_all")}
          tour_anchor={SKILLS_TOUR_ANCHORS.categories}
          value={ctrl.active_category}
        />
      ) : null}
    </div>
  );
}
