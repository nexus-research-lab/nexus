"use client";

import { Filter, Users } from "lucide-react";

import {
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
} from "@/features/capability/shared/capability-page-layout";
import type { ImChannelType } from "@/lib/api/capability/channel-api";
import { cn } from "@/shared/ui/class-name";
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
  return (
    <div className="mb-5">
      <div className="overflow-x-auto border-b border-(--divider-subtle-color)">
        <div
          aria-label="按配对状态筛选"
          className="flex min-w-max items-end gap-1"
          role="tablist"
        >
          {STATUS_TABS.map((tab) => {
            const selected = filters.status === tab.value;
            return (
              <button
                aria-selected={selected}
                className={cn(
                  "flex h-10 items-center gap-2 border-b-2 px-3 text-[13px] font-semibold transition-colors",
                  selected
                    ? "border-(--primary) text-(--text-strong)"
                    : "border-transparent text-(--text-muted) hover:text-(--text-default)",
                )}
                key={tab.value || "all"}
                onClick={() => onChange("status", tab.value)}
                role="tab"
                type="button"
              >
                <span>{tab.label}</span>
                <span className="min-w-4 text-right text-[11px] tabular-nums text-(--text-soft)">
                  {counts[tab.countKey]}
                </span>
              </button>
            );
          })}
        </div>
      </div>

      <div className="mt-3 flex w-full flex-col gap-2.5 sm:flex-row sm:items-center">
        <CapabilityFilterSearchInput
          onChange={(value) => onChange("query", value)}
          placeholder={searchPlaceholder}
          value={filters.query}
        />
        <CapabilityFilterSelect
          ariaLabel="按渠道筛选"
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
    </div>
  );
}
