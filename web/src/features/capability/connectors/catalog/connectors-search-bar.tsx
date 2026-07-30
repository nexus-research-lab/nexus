"use client";

import { SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";

import { CONNECTOR_CATEGORY_OPTIONS, getConnectorCategoryLabel } from "./connectors-categories";

interface ConnectorsSearchBarProps {
  activeCategory: string;
  onCategoryChange: (category: string) => void;
  onQueryChange: (query: string) => void;
  searchQuery: string;
}

export function ConnectorsSearchBar({
  activeCategory,
  onCategoryChange,
  onQueryChange,
  searchQuery,
}: ConnectorsSearchBarProps) {
  const { t } = useI18n();

  return (
    <CapabilityFilterBar>
      <CapabilityFilterSearchInput
        onChange={onQueryChange}
        placeholder={t("capability.connectors_search_placeholder")}
        value={searchQuery}
      />
      <CapabilityFilterSelect
        ariaLabel={t("capability.connectors_filter_aria")}
        label={t("capability.category_label")}
        leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
        onChange={onCategoryChange}
        options={CONNECTOR_CATEGORY_OPTIONS.map((item) => ({
          label: t(item.labelKey),
          value: item.key,
        }))}
        placeholder={getConnectorCategoryLabel("all", t)}
        value={activeCategory}
      />
    </CapabilityFilterBar>
  );
}
