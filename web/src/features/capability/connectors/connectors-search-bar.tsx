"use client";

import { SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";

import { CONNECTOR_CATEGORY_OPTIONS, getConnectorCategoryLabel } from "./connectors-categories";
import type { ConnectorDirectoryController } from "./connectors-view-model";

interface ConnectorsSearchBarProps {
  ctrl: ConnectorDirectoryController;
}

export function ConnectorsSearchBar({ ctrl }: ConnectorsSearchBarProps) {
  const { t } = useI18n();

  return (
    <div className="mb-5 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
      <CapabilityFilterSearchInput
        onChange={ctrl.setSearchQuery}
        placeholder={t("capability.connectors_search_placeholder")}
        value={ctrl.searchQuery}
      />
      <CapabilityFilterSelect
        ariaLabel={t("capability.connectors_filter_aria")}
        label={t("capability.category_label")}
        leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
        onChange={ctrl.setActiveCategory}
        options={CONNECTOR_CATEGORY_OPTIONS.map((item) => ({
          label: t(item.labelKey),
          value: item.key,
        }))}
        placeholder={getConnectorCategoryLabel("all", t)}
        value={ctrl.activeCategory}
      />
    </div>
  );
}
