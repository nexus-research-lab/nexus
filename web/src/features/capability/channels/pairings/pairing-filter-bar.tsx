// INPUT: 配对目录计数、搜索及渠道/Agent 筛选命令。
// OUTPUT: 公共目录页签与统一标签筛选器；Agent 同名/缺项文字复用公共选项投影。
// POS: Pairing 工具区纯视图；不拥有筛选图标或菜单 DOM。
"use client";

import { UiFilterSelect } from "@/shared/ui/menu/filter-select";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
} from "@/features/capability/shared/capability-page-layout";
import type { ImChannelType } from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiDirectoryTabs } from "@/shared/ui/navigation/directory-tabs";
import type { Agent } from "@/types/agent/agent";
import { buildAgentSelectionOptions, includeUnavailableAgentSelection } from "@/lib/agent-selection-options";

import type {
  PairingFilters,
  PairingStatusCounts,
  PairingStatusFilter,
} from "./pairing-model";
import type { TranslationKey } from "@/shared/i18n/messages";
import { getPairingOptions } from "./pairing-options";

interface PairingFilterBarProps {
  agents: Agent[];
  counts: PairingStatusCounts;
  filters: PairingFilters;
  onChange: <Key extends keyof PairingFilters>(
    key: Key,
    value: PairingFilters[Key],
  ) => void;
  searchPlaceholder: string;
}

interface StatusTab {
  countKey: keyof PairingStatusCounts;
  labelKey: TranslationKey;
  value: PairingStatusFilter;
}

const STATUS_TABS: StatusTab[] = [
  { countKey: "all", labelKey: "capability.pairing_status_all", value: "" },
  { countKey: "pending", labelKey: "capability.pairing_status_pending", value: "pending" },
  { countKey: "active", labelKey: "capability.pairing_status_active", value: "active" },
  { countKey: "inactive", labelKey: "capability.pairing_status_disabled", value: "inactive" },
];

export function PairingFilterBar({
  agents,
  counts,
  filters,
  onChange,
  searchPlaceholder,
}: PairingFilterBarProps) {
  const { t } = useI18n();

  return (
    <CapabilityFilterBar className="mb-5 sm:justify-between">
      <UiDirectoryTabs
        activeValue={filters.status}
        ariaLabel={t("capability.pairing_filter_status")}
        onChange={(value) => onChange("status", value)}
        options={STATUS_TABS.map((tab) => ({
          label: (
            <>
              <span>{t(tab.labelKey)}</span>
              <span className="min-w-4 text-right tabular-nums text-(--text-soft)">
                {counts[tab.countKey]}
              </span>
            </>
          ),
          value: tab.value,
        }))}
      />

      <div className="flex min-w-0 flex-1 flex-col gap-2 sm:ml-auto sm:max-w-[720px] sm:flex-row sm:items-center">
        <CapabilityFilterSearchInput
          onChange={(value) => onChange("query", value)}
          placeholder={searchPlaceholder}
          value={filters.query}
        />
        <UiFilterSelect
          ariaLabel={t("capability.pairing_filter_channel")}
          onChange={(value) => onChange(
            "channel",
            value as ImChannelType | "",
          )}
          options={[
            { value: "", label: t("capability.pairing_all_channels") },
            ...getPairingOptions(t).channels,
          ]}
          value={filters.channel}
        />
        <UiFilterSelect
          ariaLabel={t("capability.pairing_filter_agent")}
          className="sm:w-[220px]"
          onChange={(value) => onChange("agentId", value)}
          options={[
            { value: "", label: t("capability.pairing_all_agents") },
            ...includeUnavailableAgentSelection(buildAgentSelectionOptions(agents, t), filters.agentId, t),
          ]}
          value={filters.agentId}
        />
      </div>
    </CapabilityFilterBar>
  );
}
