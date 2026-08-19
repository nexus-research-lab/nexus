import { Search, SlidersHorizontal } from "lucide-react";
import { useRef, type KeyboardEvent } from "react";

import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { cn } from "@/shared/ui/class-name";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import type { DiscoveryMode } from "./controller/skill-marketplace-controller";

const DISCOVERY_OPTIONS: ReadonlyArray<{
  labelKey: TranslationKey;
  value: DiscoveryMode;
}> = [
  { labelKey: "capability.skills_tab_catalog", value: "catalog" },
  { labelKey: "capability.skills_tab_external", value: "external" },
];

interface SkillsSearchBarProps {
  activeCategory: string;
  catalogQuery: string;
  categories: Array<{ key: string; label: string }>;
  discoveryMode: DiscoveryMode;
  externalLoading: boolean;
  externalQuery: string;
  externalSourceId: string;
  externalSources: Array<{ disabled?: boolean; label: string; value: string }>;
  onChangeCategory: (category: string) => void;
  onChangeCatalogQuery: (query: string) => void;
  onChangeDiscoveryMode: (mode: DiscoveryMode) => void;
  onChangeExternalQuery: (query: string) => void;
  onChangeExternalSource: (sourceId: string) => void;
  onSubmitExternalSearch: () => void;
}

export function SkillsSearchBar({
  activeCategory,
  catalogQuery,
  categories,
  discoveryMode,
  externalLoading,
  externalQuery,
  externalSourceId,
  externalSources,
  onChangeCategory,
  onChangeCatalogQuery,
  onChangeDiscoveryMode,
  onChangeExternalQuery,
  onChangeExternalSource,
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
      disabled={externalQuery.trim().length < 2 || externalLoading}
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
    <CapabilityFilterBar className="sm:justify-between">
      <div
        aria-label={t("capability.skills_tour_modes_title")}
        className="inline-flex h-8 w-full shrink-0 items-center gap-1 sm:w-auto"
        data-tour-anchor={SKILLS_TOUR_ANCHORS.modes}
        role="group"
      >
        {DISCOVERY_OPTIONS.map((option) => {
          const active = discoveryMode === option.value;
          return (
            <button
              key={option.value}
              aria-pressed={active}
              className={cn(
                "inline-flex h-8 min-w-0 flex-1 items-center justify-center rounded-[8px] px-3 text-xs font-medium leading-none transition-[background,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] sm:flex-none",
                active
                  ? "bg-(--surface-interactive-active-background) font-semibold text-(--text-strong)"
                  : "text-(--text-muted) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
              )}
              onClick={() => onChangeDiscoveryMode(option.value)}
              type="button"
            >
              {t(option.labelKey)}
            </button>
          );
        })}
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-2 sm:ml-auto sm:max-w-[520px] sm:flex-row sm:items-center">
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
        ) : (
          <CapabilityFilterSelect
            ariaLabel={t("capability.skill_source_search_scope")}
            className="sm:w-[176px]"
            label={t("capability.skill_sources")}
            onChange={onChangeExternalSource}
            options={externalSources}
            value={externalSourceId}
          />
        )}
      </div>
    </CapabilityFilterBar>
  );
}
