import {
  Calendar,
  Link2,
  type LucideIcon,
  Puzzle,
  Radio,
  Users2,
  GitBranchPlus,
} from "lucide-react";

import { AppRouteBuilders } from "@/shared/navigation/route-paths";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { SIDEBAR_CAPABILITY_ITEM_IDS } from "@/store/sidebar";

interface CapabilitySidebarDefinition {
  icon: LucideIcon;
  id: string;
  labelKey: TranslationKey;
  path: string;
}

export interface CapabilitySidebarItem {
  icon: LucideIcon;
  id: string;
  label: string;
  path: string;
}

const CAPABILITY_SIDEBAR_DEFINITIONS: readonly CapabilitySidebarDefinition[] = [
  {
    icon: Puzzle,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.skills,
    labelKey: "capability.skills",
    path: AppRouteBuilders.skills(),
  },
  {
    icon: GitBranchPlus,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.workGraphDistillations,
    labelKey: "capability.workgraph_distillations",
    path: AppRouteBuilders.workGraphDistillations(),
  },
  {
    icon: Link2,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.connectors,
    labelKey: "capability.connectors",
    path: AppRouteBuilders.connectors(),
  },
  {
    icon: Calendar,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.scheduledTasks,
    labelKey: "capability.scheduled",
    path: AppRouteBuilders.scheduledTasks(),
  },
  {
    icon: Radio,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.channels,
    labelKey: "capability.channels",
    path: AppRouteBuilders.channels(),
  },
  {
    icon: Users2,
    id: SIDEBAR_CAPABILITY_ITEM_IDS.pairings,
    labelKey: "capability.pairings",
    path: AppRouteBuilders.pairings(),
  },
];

export function buildCapabilitySidebarItems(
  translate: I18nContextValue["t"],
): CapabilitySidebarItem[] {
  return CAPABILITY_SIDEBAR_DEFINITIONS.map((definition) => ({
    icon: definition.icon,
    id: definition.id,
    label: translate(definition.labelKey),
    path: definition.path,
  }));
}

export function filterCapabilitySidebarItems(
  items: CapabilitySidebarItem[],
  query: string,
): CapabilitySidebarItem[] {
  const search = createUiSearchMatcher(query);
  return items.filter((item) => search.matches([item.label]));
}
