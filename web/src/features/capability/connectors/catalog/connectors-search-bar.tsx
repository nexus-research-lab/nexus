// INPUT: Connector 目录模式与搜索状态。
// OUTPUT: 使用 Capability 公共筛选组件的目录模式与搜索；分类由目录分区标题表达，不再提供分类下拉。
// POS: Connector 目录筛选纯视图。

"use client";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiDirectoryTabs } from "@/shared/ui/navigation/directory-tabs";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
} from "@/features/capability/shared/capability-page-layout";

export type ConnectorDirectoryMode = "catalog" | "custom_mcp";

interface ConnectorsSearchBarProps {
  onModeChange: (mode: ConnectorDirectoryMode) => void;
  onQueryChange: (query: string) => void;
  mode: ConnectorDirectoryMode;
  searchQuery: string;
}

export function ConnectorsSearchBar({
  onModeChange,
  onQueryChange,
  mode,
  searchQuery,
}: ConnectorsSearchBarProps) {
  const { t } = useI18n();

  return (
    <CapabilityFilterBar className="sm:justify-between">
      <UiDirectoryTabs
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
      </div>
    </CapabilityFilterBar>
  );
}
