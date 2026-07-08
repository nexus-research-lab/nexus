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
  const composingRef = useRef(false);
  const searchLabel = t("capability.skills_tour_search_title");

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (ctrl.discoveryMode !== "external") return;
    if (event.key !== "Enter") return;
    if (composingRef.current || event.nativeEvent.isComposing) return;
    event.preventDefault();
    ctrl.submitExternalSearch();
  };

  const externalSearchAction = ctrl.discoveryMode === "external" ? (
    <button
      aria-label={searchLabel}
      className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) text-(--text-muted) transition hover:border-(--primary) hover:text-(--primary) disabled:pointer-events-none disabled:opacity-45"
      disabled={!ctrl.externalQuery.trim() || ctrl.externalLoading}
      onClick={(event) => {
        event.preventDefault();
        ctrl.submitExternalSearch();
      }}
      onMouseDown={(event) => event.preventDefault()}
      title={searchLabel}
      type="button"
    >
      <Search className="h-3.5 w-3.5" />
    </button>
  ) : null;

  return (
    <div className="mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
      <CapabilityFilterSearchInput
        action={externalSearchAction}
        onChange={(value) => {
          if (ctrl.discoveryMode === "catalog") {
            ctrl.setSearchQuery(value);
            return;
          }
          ctrl.setExternalQuery(value);
        }}
        onCompositionEnd={() => {
          composingRef.current = false;
        }}
        onCompositionStart={() => {
          composingRef.current = true;
        }}
        onKeyDown={handleKeyDown}
        placeholder={
          ctrl.discoveryMode === "catalog"
            ? t("capability.skills_search_catalog")
            : t("capability.skills_search_external")
        }
        value={ctrl.discoveryMode === "catalog" ? ctrl.searchQuery : ctrl.externalQuery}
      />

      {ctrl.discoveryMode === "catalog" ? (
        <CapabilityFilterSelect
          ariaLabel={t("capability.skills_filter_aria")}
          label={t("capability.category_label")}
          leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
          onChange={ctrl.setActiveCategory}
          options={ctrl.categories.map((category) => ({
            label: category.label,
            value: category.key,
          }))}
          placeholder={t("capability.category_all")}
          tourAnchor={SKILLS_TOUR_ANCHORS.categories}
          value={ctrl.activeCategory}
        />
      ) : null}
    </div>
  );
}
