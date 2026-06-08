import { Search, SlidersHorizontal } from "lucide-react";
import { useRef, type KeyboardEvent } from "react";

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
  const composing_ref = useRef(false);
  const search_label = t("capability.skills_tour_search_title");

  const handle_key_down = (event: KeyboardEvent<HTMLInputElement>) => {
    if (ctrl.discovery_mode !== "external") return;
    if (event.key !== "Enter") return;
    if (composing_ref.current || event.nativeEvent.isComposing) return;
    event.preventDefault();
    ctrl.submit_external_search();
  };

  const external_search_action = ctrl.discovery_mode === "external" ? (
    <button
      aria-label={search_label}
      className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) text-(--text-muted) transition hover:border-(--primary) hover:text-(--primary) disabled:pointer-events-none disabled:opacity-45"
      disabled={!ctrl.external_query.trim() || ctrl.external_loading}
      onClick={(event) => {
        event.preventDefault();
        ctrl.submit_external_search();
      }}
      onMouseDown={(event) => event.preventDefault()}
      title={search_label}
      type="button"
    >
      <Search className="h-3.5 w-3.5" />
    </button>
  ) : null;

  return (
    <div className="mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
      <CapabilityFilterSearchInput
        action={external_search_action}
        on_change={(value) => {
          if (ctrl.discovery_mode === "catalog") {
            ctrl.set_search_query(value);
            return;
          }
          ctrl.set_external_query(value);
        }}
        on_composition_end={() => {
          composing_ref.current = false;
        }}
        on_composition_start={() => {
          composing_ref.current = true;
        }}
        on_key_down={handle_key_down}
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
