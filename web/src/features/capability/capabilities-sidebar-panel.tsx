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
  Database,
  Link2,
  type LucideIcon,
  Puzzle,
  Radio,
  Users2,
} from "lucide-react";
import { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_capability_summary_api, type CapabilitySummary } from "@/lib/api/capability-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSearchInput } from "@/shared/ui/form-control";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SIDEBAR_CAPABILITY_ITEM_IDS, useSidebarStore } from "@/store/sidebar";

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";
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
  const active_panel_item_id = useSidebarStore((s) => s.active_panel_item_id);
  const set_active_panel_item = useSidebarStore((s) => s.set_active_panel_item);
  const summary_mounted_ref = useRef(false);
  const summary_refresh_in_flight_ref = useRef(false);
  const summary_pending_force_refresh_ref = useRef(false);
  const summary_last_refreshed_at_ref = useRef(0);
  const [query, set_query] = useState("");
  const [summary, set_summary] = useState<CapabilitySummary>({
    skills_count: 0,
    connected_connectors_count: 0,
    enabled_scheduled_tasks_count: 0,
    configured_channels_count: 0,
    active_pairings_count: 0,
  });

  const refresh_capability_summary = useCallback(async (options?: { force?: boolean; reset_on_error?: boolean }) => {
    if (summary_refresh_in_flight_ref.current) {
      if (options?.force) {
        summary_pending_force_refresh_ref.current = true;
      }
      return;
    }

    let next_refresh_options = options;
    do {
      // focus/visibility 在桌面壳里可能连续触发，摘要计数不需要每次都打后端。
      if (
        !next_refresh_options?.force &&
        Date.now() - summary_last_refreshed_at_ref.current < CAPABILITY_SUMMARY_REVALIDATE_INTERVAL_MS
      ) {
        return;
      }
      summary_refresh_in_flight_ref.current = true;
      summary_pending_force_refresh_ref.current = false;
      summary_last_refreshed_at_ref.current = Date.now();
      try {
        const next_summary = await get_capability_summary_api();
        if (summary_mounted_ref.current) {
          set_summary(next_summary);
        }
      } catch {
        if (next_refresh_options?.reset_on_error && summary_mounted_ref.current) {
          set_summary({
            skills_count: 0,
            connected_connectors_count: 0,
            enabled_scheduled_tasks_count: 0,
            configured_channels_count: 0,
            active_pairings_count: 0,
          });
        }
      } finally {
        summary_refresh_in_flight_ref.current = false;
      }

      const should_run_pending_force_refresh = summary_pending_force_refresh_ref.current;
      summary_pending_force_refresh_ref.current = false;
      next_refresh_options = should_run_pending_force_refresh && summary_mounted_ref.current
        ? { force: true }
        : undefined;
    } while (next_refresh_options);
  }, []);

  useEffect(() => {
    summary_mounted_ref.current = true;
    void refresh_capability_summary({ force: true, reset_on_error: true });

    const handle_scheduled_tasks_mutated = () => {
      void refresh_capability_summary({ force: true });
    };
    window.addEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handle_scheduled_tasks_mutated);

    return () => {
      summary_mounted_ref.current = false;
      window.removeEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handle_scheduled_tasks_mutated);
    };
  }, [refresh_capability_summary]);

  useEffect(() => {
    if (typeof window === "undefined") {
      return;
    }
    const handle_revalidate = () => {
      if (document.visibilityState !== "visible") {
        return;
      }
      void refresh_capability_summary();
    };
    window.addEventListener("focus", handle_revalidate);
    document.addEventListener("visibilitychange", handle_revalidate);
    return () => {
      window.removeEventListener("focus", handle_revalidate);
      document.removeEventListener("visibilitychange", handle_revalidate);
    };
  }, [refresh_capability_summary]);

  const channel_count = summary.configured_channels_count ?? 0;
  const pairing_count = summary.active_pairings_count ?? 0;
  const capability_items = useMemo<CapabilitySidebarItem[]>(() => [
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
      icon: Puzzle,
      label: t("capability.skills"),
      meta: String(summary.skills_count),
      path: AppRouteBuilders.skills(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.connectors,
      icon: Link2,
      label: t("capability.connectors"),
      meta: String(summary.connected_connectors_count),
      path: AppRouteBuilders.connectors(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.scheduled_tasks,
      icon: Calendar,
      label: t("capability.scheduled"),
      meta: String(summary.enabled_scheduled_tasks_count),
      path: AppRouteBuilders.scheduled_tasks(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.channels,
      icon: Radio,
      label: t("capability.channels"),
      meta: String(channel_count),
      path: AppRouteBuilders.channels(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.pairings,
      icon: Users2,
      label: t("capability.pairings"),
      meta: String(pairing_count),
      path: AppRouteBuilders.pairings(),
    },
    {
      id: SIDEBAR_CAPABILITY_ITEM_IDS.memory,
      icon: Database,
      label: t("capability.memory"),
      meta: "v1",
      path: AppRouteBuilders.memory(),
    },
  ], [
    channel_count,
    pairing_count,
    summary,
    t,
  ]);

  const filtered_capability_items = useMemo(() => {
    const normalized_query = query.trim().toLowerCase();
    if (!normalized_query) {
      return capability_items;
    }
    return capability_items.filter((item) =>
      `${item.label} ${item.meta}`.toLowerCase().includes(normalized_query),
    );
  }, [capability_items, query]);

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="pb-2">
        <UiSearchInput
          class_name="w-full"
          input_class_name="text-[13px]"
          on_change={set_query}
          placeholder={t("sidebar.search_capabilities")}
          value={query}
        />
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-1">
        {filtered_capability_items.length > 0 ? (
          filtered_capability_items.map((item) => {
            const Icon = item.icon;
            return (
              <SidebarListItem
                icon={<Icon className="h-4 w-4" />}
                is_active={active_panel_item_id === item.id}
                key={item.id}
                label={item.label}
                meta={item.meta}
                on_click={() => {
                  set_active_panel_item(item.id);
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
