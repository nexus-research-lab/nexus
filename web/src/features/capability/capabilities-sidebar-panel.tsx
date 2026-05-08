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
  Users2,
} from "lucide-react";
import { Fragment, memo, useCallback, useEffect, useMemo, useState } from "react";
import { useNavigate } from "react-router-dom";

import { AppRouteBuilders } from "@/app/router/route-paths";
import { get_capability_summary_api, type CapabilitySummary } from "@/lib/api/capability-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarListItem } from "@/shared/ui/sidebar/collapsible-section";
import { SIDEBAR_CAPABILITY_ITEM_IDS, useSidebarStore } from "@/store/sidebar";

const SCHEDULED_TASKS_MUTATED_EVENT = "nexus:scheduled-tasks-mutated";

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
  const [summary, set_summary] = useState<CapabilitySummary>({
    skills_count: 0,
    connected_connectors_count: 0,
    enabled_scheduled_tasks_count: 0,
    configured_channels_count: 0,
    active_pairings_count: 0,
  });

  const refresh_capability_summary = useCallback(async (options?: { reset_on_error?: boolean }) => {
    try {
      const next_summary = await get_capability_summary_api();
      set_summary(next_summary);
    } catch {
      if (options?.reset_on_error) {
        set_summary({
          skills_count: 0,
          connected_connectors_count: 0,
          enabled_scheduled_tasks_count: 0,
          configured_channels_count: 0,
          active_pairings_count: 0,
        });
      }
    }
  }, []);

  useEffect(() => {
    let cancelled = false;
    const refresh_if_mounted = async () => {
      try {
        const next_summary = await get_capability_summary_api();
        if (!cancelled) {
          set_summary(next_summary);
        }
      } catch {
        if (!cancelled) {
          set_summary({
            skills_count: 0,
            connected_connectors_count: 0,
            enabled_scheduled_tasks_count: 0,
            configured_channels_count: 0,
            active_pairings_count: 0,
          });
        }
      }
    };
    void refresh_if_mounted();

    const handle_scheduled_tasks_mutated = () => {
      void refresh_capability_summary();
    };
    window.addEventListener(SCHEDULED_TASKS_MUTATED_EVENT, handle_scheduled_tasks_mutated);

    return () => {
      cancelled = true;
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
    const interval_id = window.setInterval(handle_revalidate, 15000);
    return () => {
      window.removeEventListener("focus", handle_revalidate);
      document.removeEventListener("visibilitychange", handle_revalidate);
      window.clearInterval(interval_id);
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
  ], [
    channel_count,
    pairing_count,
    summary,
    t,
  ]);

  return (
    <Fragment>
      {capability_items.map((item) => {
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
      })}
    </Fragment>
  );
});
