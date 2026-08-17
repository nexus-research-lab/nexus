"use client";

import { Filter, Users } from "lucide-react";

import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import type { ImChannelType } from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiTabs } from "@/shared/ui/navigation/tabs";
import type { Agent } from "@/types/agent/agent";

import type {
  PairingFilters,
  PairingStatusCounts,
  PairingStatusFilter,
} from "./pairing-model";
import { CHANNEL_OPTIONS } from "./pairing-options";

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
  label: string;
  value: PairingStatusFilter;
}

const STATUS_TABS: StatusTab[] = [
  { countKey: "all", label: "全部", value: "" },
  { countKey: "pending", label: "待处理", value: "pending" },
  { countKey: "active", label: "已授权", value: "active" },
  { countKey: "inactive", label: "已停用", value: "inactive" },
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
      <UiTabs
        activeValue={filters.status}
        ariaLabel="按配对状态筛选"
        className="h-8 w-full shrink-0 sm:w-auto"
        density="compact"
        itemClassName="h-8 w-full justify-center px-3 sm:w-auto"
        onChange={(value) => onChange("status", value)}
        options={STATUS_TABS.map((tab) => ({
          className: "min-w-0 flex-1 sm:flex-none",
          label: (
            <>
              <span>{tab.label}</span>
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
        <CapabilityFilterSelect
          ariaLabel="按渠道筛选"
          label={t("capability.channel_label")}
          leading={<Filter className="h-3.5 w-3.5" />}
          onChange={(value) => onChange(
            "channel",
            value as ImChannelType | "",
          )}
          options={[
            { value: "", label: "全部渠道" },
            ...CHANNEL_OPTIONS,
          ]}
          value={filters.channel}
        />
        <CapabilityFilterSelect
          ariaLabel="按处理智能体筛选"
          className="sm:w-[220px]"
          label={t("capability.agent_label")}
          leading={<Users className="h-3.5 w-3.5" />}
          onChange={(value) => onChange("agentId", value)}
          options={[
            { value: "", label: "全部智能体" },
            ...agents.map((agent) => ({
              value: agent.agent_id,
              label: agent.name,
            })),
          ]}
          value={filters.agentId}
        />
      </div>
    </CapabilityFilterBar>
  );
}
