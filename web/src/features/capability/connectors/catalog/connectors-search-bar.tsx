"use client";

import { SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import { UiTabs } from "@/shared/ui/navigation/tabs";

import { CONNECTOR_CATEGORY_OPTIONS, getConnectorCategoryLabel } from "./connectors-categories";

export type ConnectorDirectoryMode = "catalog" | "custom_mcp";

interface ConnectorsSearchBarProps {
  activeCategory: string;
  onCategoryChange: (category: string) => void;
  onModeChange: (mode: ConnectorDirectoryMode) => void;
  onQueryChange: (query: string) => void;
  mode: ConnectorDirectoryMode;
  searchQuery: string;
}

export function ConnectorsSearchBar({
  activeCategory,
  onCategoryChange,
  onModeChange,
  onQueryChange,
  mode,
  searchQuery,
}: ConnectorsSearchBarProps) {
  const { t } = useI18n();

  return (
    <CapabilityFilterBar className="sm:justify-between">
      <UiTabs
        activeValue={mode}
        ariaLabel={t("capability.connectors_modes_aria")}
        className="h-8 w-full shrink-0 sm:w-auto"
        density="compact"
        itemClassName="h-8 w-full justify-center px-3 sm:w-auto"
        onChange={onModeChange}
        options={[
          {
            className: "min-w-0 flex-1 sm:flex-none",
            label: t("capability.connectors_tab_catalog"),
            value: "catalog",
          },
          {
            className: "min-w-0 flex-1 sm:flex-none",
            label: t("capability.connectors_tab_custom_mcp"),
            value: "custom_mcp",
          },
        ]}
      />

      <div className="flex min-w-0 flex-1 flex-col gap-2 sm:ml-auto sm:max-w-[520px] sm:flex-row sm:items-center">
        <CapabilityFilterSearchInput
          onChange={onQueryChange}
          placeholder={mode === "catalog"
            ? t("capability.connectors_search_placeholder")
            : t("capability.custom_mcp_search_placeholder")}
          value={searchQuery}
        />
        {mode === "catalog" ? (
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
        ) : null}
      </div>
    </CapabilityFilterBar>
  );
}
