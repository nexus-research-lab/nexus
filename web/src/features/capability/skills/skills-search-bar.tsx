import { Search, SlidersHorizontal } from "lucide-react";
import { useRef, type KeyboardEvent } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import type { DiscoveryMode } from "./controller/skill-marketplace-controller";
import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";

interface SkillsSearchBarProps {
  activeCategory: string;
  catalogQuery: string;
  categories: Array<{ key: string; label: string }>;
  discoveryMode: DiscoveryMode;
  externalLoading: boolean;
  externalQuery: string;
  onChangeCategory: (category: string) => void;
  onChangeCatalogQuery: (query: string) => void;
  onChangeExternalQuery: (query: string) => void;
  onSubmitExternalSearch: () => void;
}

export function SkillsSearchBar({
  activeCategory,
  catalogQuery,
  categories,
  discoveryMode,
  externalLoading,
  externalQuery,
  onChangeCategory,
  onChangeCatalogQuery,
  onChangeExternalQuery,
  onSubmitExternalSearch,
}: SkillsSearchBarProps) {
  const { t } = useI18n();
  const composingRef = useRef(false);
  const searchLabel = t("capability.skills_tour_search_title");

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (discoveryMode !== "external") return;
    if (event.key !== "Enter") return;
    if (composingRef.current || event.nativeEvent.isComposing) return;
    event.preventDefault();
    onSubmitExternalSearch();
  };

  const externalSearchAction = discoveryMode === "external" ? (
    <button
      aria-label={searchLabel}
      className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) text-(--text-muted) transition hover:border-(--primary) hover:text-(--primary) disabled:pointer-events-none disabled:opacity-45"
      disabled={!externalQuery.trim() || externalLoading}
      onClick={(event) => {
        event.preventDefault();
        onSubmitExternalSearch();
      }}
      onMouseDown={(event) => event.preventDefault()}
      title={searchLabel}
      type="button"
    >
      <Search className="h-3.5 w-3.5" />
    </button>
  ) : null;

  return (
    <div className="mb-4 flex w-full flex-col gap-2 sm:flex-row sm:items-center">
      <CapabilityFilterSearchInput
        action={externalSearchAction}
        onChange={(value) => {
          if (discoveryMode === "catalog") {
            onChangeCatalogQuery(value);
            return;
          }
          onChangeExternalQuery(value);
        }}
        onCompositionEnd={() => {
          composingRef.current = false;
        }}
        onCompositionStart={() => {
          composingRef.current = true;
        }}
        onKeyDown={handleKeyDown}
        placeholder={
          discoveryMode === "catalog"
            ? t("capability.skills_search_catalog")
            : t("capability.skills_search_external")
        }
        value={discoveryMode === "catalog" ? catalogQuery : externalQuery}
      />

      {discoveryMode === "catalog" ? (
        <CapabilityFilterSelect
          ariaLabel={t("capability.skills_filter_aria")}
          label={t("capability.category_label")}
          leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
          onChange={onChangeCategory}
          options={categories.map((category) => ({
            label: category.label,
            value: category.key,
          }))}
          placeholder={t("capability.category_all")}
          tourAnchor={SKILLS_TOUR_ANCHORS.categories}
          value={activeCategory}
        />
      ) : null}
    </div>
  );
}
