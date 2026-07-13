import {
  Calendar,
  Link2,
  type LucideIcon,
  Puzzle,
  Radio,
  Repeat2,
  Users2,
} from "lucide-react";

import { AppRouteBuilders } from "@/app/router/route-paths";
import type { CapabilitySummary } from "@/lib/api/capability/summary-api";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { SIDEBAR_CAPABILITY_ITEM_IDS } from "@/store/sidebar";

interface CapabilitySidebarDefinition {
  countKey: keyof CapabilitySummary;
  icon: LucideIcon;
  id: string;
  labelKey: TranslationKey;
  path: string;
}

export interface CapabilitySidebarItem {
  icon: LucideIcon;
  id: string;
  label: string;
  meta: string;
  path: string;
}

const CAPABILITY_SIDEBAR_DEFINITIONS: readonly CapabilitySidebarDefinition[] = [
  {
    countKey: "skills_count",
    icon: Puzzle,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
    labelKey: "capability.skills",
    path: AppRouteBuilders.skills(),
  },
  {
    countKey: "loops_count",
    icon: Repeat2,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.loops,
    labelKey: "capability.loops",
    path: AppRouteBuilders.loops(),
  },
  {
    countKey: "connected_connectors_count",
    icon: Link2,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.connectors,
    labelKey: "capability.connectors",
    path: AppRouteBuilders.connectors(),
  },
  {
    countKey: "enabled_scheduled_tasks_count",
    icon: Calendar,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.scheduledTasks,
    labelKey: "capability.scheduled",
    path: AppRouteBuilders.scheduledTasks(),
  },
  {
    countKey: "connected_channels_count",
    icon: Radio,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.channels,
    labelKey: "capability.channels",
    path: AppRouteBuilders.channels(),
  },
  {
    countKey: "active_pairings_count",
    icon: Users2,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.pairings,
    labelKey: "capability.pairings",
    path: AppRouteBuilders.pairings(),
  },
];

export function buildCapabilitySidebarItems(
  summary: CapabilitySummary,
  translate: I18nContextValue["t"],
): CapabilitySidebarItem[] {
  return CAPABILITY_SIDEBAR_DEFINITIONS.map((definition) => ({
    icon: definition.icon,
    id: definition.id,
    label: translate(definition.labelKey),
    meta: String(summary[definition.countKey]),
    path: definition.path,
  }));
}

export function filterCapabilitySidebarItems(
  items: CapabilitySidebarItem[],
  query: string,
): CapabilitySidebarItem[] {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) {
    return items;
  }
  return items.filter((item) =>
    `${item.label} ${item.meta}`.toLocaleLowerCase().includes(normalizedQuery),
  );
}
