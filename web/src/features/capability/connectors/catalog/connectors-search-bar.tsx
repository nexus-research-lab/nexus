// INPUT: Connector 目录模式、当前可用分类键、搜索与筛选状态。
// OUTPUT: 使用 Capability 公共筛选组件的目录模式、搜索和有效分类控件。
// POS: Connector 目录筛选纯视图；分类集合由目录模型提供，不展示空分类。

"use client";

import { SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  CapabilityFilterBar,
  CapabilityDirectoryTabs,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";

import { getConnectorCategoryLabel } from "./connectors-categories";

export type ConnectorDirectoryMode = "catalog" | "custom_mcp";

interface ConnectorsSearchBarProps {
  activeCategory: string;
  categoryKeys: string[];
  onCategoryChange: (category: string) => void;
  onModeChange: (mode: ConnectorDirectoryMode) => void;
  onQueryChange: (query: string) => void;
  mode: ConnectorDirectoryMode;
  searchQuery: string;
}

export function ConnectorsSearchBar({
  activeCategory,
  categoryKeys,
  onCategoryChange,
  onModeChange,
  onQueryChange,
  mode,
  searchQuery,
}: ConnectorsSearchBarProps) {
  const { t } = useI18n();

  return (
    <CapabilityFilterBar className="sm:justify-between">
      <CapabilityDirectoryTabs
        activeValue={mode}
        ariaLabel={t("capability.connectors_modes_aria")}
        onChange={onModeChange}
        options={[
          {
            label: t("capability.connectors_tab_catalog"),
            value: "catalog",
          },
          {
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
            options={["all", ...categoryKeys].map((category) => ({
              label: getConnectorCategoryLabel(category, t),
              value: category,
            }))}
            placeholder={getConnectorCategoryLabel("all", t)}
            value={activeCategory}
          />
        ) : null}
      </div>
    </CapabilityFilterBar>
  );
}
