/**
 * Capabilities 面板内容
 *
 * 能力分区内容。
 *
 * 这里使用和 Rooms / DMs 一致的侧栏列表形式，
 * 避免能力区仍然保持独立卡片样式。
 */

import {
  Calendar,
  Link2,
  type LucideIcon,
  Puzzle,
  Radio,
  Repeat2,
  Users2,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { getCapabilitySummaryApi, type CapabilitySummary } from "@/lib/api/capability/summary-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SIDEBAR_CAPABILITY_ITEM_IDS, useSidebarStore } from "@/store/sidebar";

import { CAPABILITY_SUMMARY_MUTATED_EVENT } from "./capability-summary-events";
import { SCHEDULED_TASKS_MUTATED_EVENT } from "./scheduled-task-events";

const CAPABILITY_SUMMARY_REVALIDATE_INTERVAL_MS = 60_000;

interface CapabilitySidebarItem {
  id: string;
  icon: LucideIcon;
  label: string;
  meta: string;
  path: string;
}

export const CapabilitiesPanelContent = memo(function CapabilitiesPanelContent() {
  const { t } = useI18n();
  const navigate = useNavigate();
  const activePanelItemId = useSidebarStore((s) => s.active_panel_item_id);
  const setActivePanelItem = useSidebarStore((s) => s.set_active_panel_item);
  const summaryMountedRef = useRef(false);
  const summaryRefreshInFlightRef = useRef(false);
  const summaryPendingForceRefreshRef = useRef(false);
  const summaryLastRefreshedAtRef = useRef(0);
  const [query, setQuery] = useState("");
  const [summary, setSummary] = useState<CapabilitySummary>({
    skills_count: 0,
    connected_connectors_count: 0,
    enabled_scheduled_tasks_count: 0,
    connected_channels_count: 0,
    configured_channels_count: 0,
    active_pairings_count: 0,
    loops_count: 0,
  });

  const refreshCapabilitySummary = useCallback(async (options?: { force?: boolean; reset_on_error?: boolean }) => {
    if (summaryRefreshInFlightRef.current) {
      if (options?.force) {
        summaryPendingForceRefreshRef.current = true;
      }
      return;
    }

    let nextRefreshOptions = options;
    do {
      // focus/visibility 在桌面壳里可能连续触发，摘要计数不需要每次都打后端。
      if (
        !nextRefreshOptions?.force &&
        Date.now() - summaryLastRefreshedAtRef.current < CAPABILITY_SUMMARY_REVALIDATE_INTERVAL_MS
      ) {
        return;
      }
      summaryRefreshInFlightRef.current = true;
      summaryPendingForceRefreshRef.current = false;
      summaryLastRefreshedAtRef.current = Date.now();
      try {
        const nextSummary = await getCapabilitySummaryApi();
        if (summaryMountedRef.current) {
          setSummary(nextSummary);
        }
      } catch {
        if (nextRefreshOptions?.reset_on_error && summaryMountedRef.current) {
          setSummary({
            skills_count: 0,
            connected_connectors_count: 0,
            enabled_scheduled_tasks_count: 0,
            connected_channels_count: 0,
            configured_channels_count: 0,
            active_pairings_count: 0,
            loops_count: 0,
          });
        }
      } finally {
        summaryRefreshInFlightRef.current = false;
      }

      const shouldRunPendingForceRefresh = summaryPendingForceRefreshRef.current;
      summaryPendingForceRefreshRef.current = false;
      nextRefreshOptions = shouldRunPendingForceRefresh && summaryMountedRef.current
        ? { force: true }
        : undefined;
    } while (nextRefreshOptions);
  }, []);

  useEffect(() => {
    summaryMountedRef.current = true;
    void refreshCapabilitySummary({ force: true, reset_on_error: true });

    const handleScheduledTasksMutated = () => {
      void refreshCapabilitySummary({ force: true });
    };
    const handleCapabilitySummaryMutated = () => {
      void refreshCapabilitySummary({ force: true });
    };
    window.addEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handleScheduledTasksMutated);
    window.addEventListener(CAPABILITY_SUMMARY_MUTATED_EVENT, handleCapabilitySummaryMutated);

    return () => {
      summaryMountedRef.current = false;
      window.removeEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handleScheduledTasksMutated);
      window.removeEventListener(CAPABILITY_SUMMARY_MUTATED_EVENT, handleCapabilitySummaryMutated);
    };
  }, [refreshCapabilitySummary]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const handleRevalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refreshCapabilitySummary();
    };
    window.addEventListener("focus", handleRevalidate);
    document.addEventListener("visibilitychange", handleRevalidate);
    return () => {
      window.removeEventListener("focus", handleRevalidate);
      document.removeEventListener("visibilitychange", handleRevalidate);
    };
  }, [refreshCapabilitySummary]);

  const channelCount = summary.connected_channels_count ?? 0;
  const pairingCount = summary.active_pairings_count ?? 0;
  const capabilityItems = useMemo<CapabilitySidebarItem[]>(() => [
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
      icon: Puzzle,
      label: t("capability.skills"),
      meta: String(summary.skills_count),
      path: AppRouteBuilders.skills(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.loops,
      icon: Repeat2,
      label: t("capability.loops"),
      meta: String(summary.loops_count ?? 0),
      path: AppRouteBuilders.loops(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.connectors,
      icon: Link2,
      label: t("capability.connectors"),
      meta: String(summary.connected_connectors_count),
      path: AppRouteBuilders.connectors(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.scheduledTasks,
      icon: Calendar,
      label: t("capability.scheduled"),
      meta: String(summary.enabled_scheduled_tasks_count),
      path: AppRouteBuilders.scheduledTasks(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.channels,
      icon: Radio,
      label: t("capability.channels"),
      meta: String(channelCount),
      path: AppRouteBuilders.channels(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.pairings,
      icon: Users2,
      label: t("capability.pairings"),
      meta: String(pairingCount),
      path: AppRouteBuilders.pairings(),
    },
  ], [
    channelCount,
    pairingCount,
    summary,
    t,
  ]);

  const filteredCapabilityItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return capabilityItems;
    }
    return capabilityItems.filter((item) =>
      `${item.label} ${item.meta}`.toLowerCase().includes(normalizedQuery),
    );
  }, [capabilityItems, query]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="pb-2">
        <UiSearchInput
          className="w-full"
          inputClassName="text-[13px]"
          onChange={setQuery}
          placeholder={t("sidebar.search_capabilities")}
          value={query}
        />
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-1">
        {filteredCapabilityItems.length > 0 ? (
          filteredCapabilityItems.map((item) => {
            const Icon = item.icon;
            return (
              <SidebarListItem
                icon={<Icon className="h-4 w-4" />}
                isActive={activePanelItemId === item.id}
                key={item.id}
                label={item.label}
                meta={item.meta}
                onClick={() => {
                  setActivePanelItem(item.id);
                  navigate(item.path);
                }}
              />
            );
          })
        ) : (
          <div className="px-2.5 py-4 text-[12px] text-(--text-muted)">
            {t("sidebar.no_matching_capabilities")}
          </div>
        )}
      </div>
    </div>
  );
});
