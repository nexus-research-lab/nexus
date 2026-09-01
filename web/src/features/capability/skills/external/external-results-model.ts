import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

export interface ExternalResultGroup {
  items: ExternalSkillSearchItem[];
  key: string;
  label: string;
  status: string;
}

type ExternalResultsPhase = "empty" | "hidden" | "loading" | "ready";

export interface ExternalResultsModel {
  groups: ExternalResultGroup[];
  phase: ExternalResultsPhase;
  selectedGroup: ExternalResultGroup | null;
  selectedSourceKey: string | null;
  showSourceFilters: boolean;
  title: string;
  visibleItems: ExternalSkillSearchItem[];
}

interface BuildExternalResultsModelOptions {
  activeSourceKey: string | null;
  items: ExternalSkillSearchItem[];
  localization: Pick<I18nContextValue, "t">;
  loading: boolean;
  statuses: ExternalSkillSourceStatus[];
  sources: ExternalSkillSourceInfo[];
  submittedQuery: string;
}

export function buildExternalResultsModel({
  activeSourceKey,
  items,
  localization,
  loading,
  statuses,
  sources,
  submittedQuery,
}: BuildExternalResultsModelOptions): ExternalResultsModel {
  const groups = hasResultContext(items, submittedQuery)
    ? groupExternalResultsBySource(items, statuses, sources, localization)
    : [];
  const selectedGroup = groups.find((group) => group.key === activeSourceKey) ?? null;
  const selectedSourceKey = selectedGroup?.key ?? null;
  return {
    groups,
    phase: resolveExternalResultsPhase(loading, submittedQuery, items, groups),
    selectedGroup,
    selectedSourceKey,
    showSourceFilters: Boolean(submittedQuery.trim()),
    title: localization.t(submittedQuery.trim()
      ? "capability.search_results"
      : "capability.skills_external_recommended"),
    visibleItems: filterAndSortExternalItems(items, selectedSourceKey),
  };
}

function groupExternalResultsBySource(
  items: ExternalSkillSearchItem[],
  statuses: ExternalSkillSourceStatus[],
  sources: ExternalSkillSourceInfo[],
  localization: Pick<I18nContextValue, "t">,
): ExternalResultGroup[] {
  const groups = new Map<string, ExternalResultGroup>();
  const statusesByKey = new Map(statuses.map((status) => [status.key, status]));
  const configuredSourceKeys = new Set<string>();

  [...sources]
    .sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name))
    .forEach((source) => {
      const status = statusesByKey.get(source.source_id);
      configuredSourceKeys.add(source.source_id);
      groups.set(source.source_id, {
        items: [],
        key: source.source_id,
        label: source.name,
        status: source.enabled ? status?.status || "ok" : "disabled",
      });
    });

  statuses.forEach((status) => {
    if (groups.has(status.key)) return;
    configuredSourceKeys.add(status.key);
    groups.set(status.key, {
      items: [],
      key: status.key,
      label: status.name,
      status: status.status,
    });
  });

  items.forEach((item) => {
    const key = externalItemSourceKey(item);
    const group = groups.get(key) ?? createResultGroup(
      item,
      key,
      localization,
    );
    group.items.push(item);
    groups.set(key, group);
  });

  return Array.from(groups.values()).filter((group) =>
    group.items.length > 0 ||
    configuredSourceKeys.has(group.key) ||
    group.status === "error" ||
    group.status === "disabled");
}

function createResultGroup(
  item: ExternalSkillSearchItem,
  key: string,
  localization: Pick<I18nContextValue, "t">,
): ExternalResultGroup {
  return {
    items: [],
    key,
    label: item.source_name || item.source_kind || localization.t(
      "capability.skills_external_source_community",
    ),
    status: "ok",
  };
}

export function sourceGroupEmptyMessage(
  group: ExternalResultGroup,
  localization: Pick<I18nContextValue, "t">,
): string {
  const messages: Record<string, string> = {
    disabled: localization.t("capability.skills_external_source_disabled_description"),
    error: localization.t("capability.skills_external_source_failed_description"),
    ok: localization.t("capability.skills_external_source_empty"),
  };
  return messages[group.status] ?? messages.ok;
}

export function sourceGroupSummaryLabel(
  group: ExternalResultGroup,
  localization: Pick<I18nContextValue, "t">,
): string {
  const labels: Record<string, string> = {
    disabled: localization.t("capability.skills_external_source_disabled"),
    error: localization.t("capability.skills_external_source_failed"),
    ok: localization.t("capability.skills_external_source_result_count", {
      count: group.items.length,
    }),
  };
  return labels[group.status] ?? labels.ok;
}

function externalItemSourceKey(item: ExternalSkillSearchItem): string {
  return item.source_key || item.source_name || item.source_kind || "community";
}

function compareExternalItems(
  left: ExternalSkillSearchItem,
  right: ExternalSkillSearchItem,
): number {
  if (left.installs !== right.installs) return right.installs - left.installs;
  const sourceOrder = (left.source_name || "").localeCompare(right.source_name || "");
  return sourceOrder || (left.title || left.name).localeCompare(right.title || right.name);
}

function hasResultContext(
  items: ExternalSkillSearchItem[],
  submittedQuery: string,
): boolean {
  return Boolean(submittedQuery.trim()) || items.length > 0;
}

function resolveExternalResultsPhase(
  loading: boolean,
  submittedQuery: string,
  items: ExternalSkillSearchItem[],
  groups: ExternalResultGroup[],
): ExternalResultsPhase {
  if (loading) return "loading";
  if (items.length || groups.length) return "ready";
  return submittedQuery ? "empty" : "hidden";
}

function filterAndSortExternalItems(
  items: ExternalSkillSearchItem[],
  sourceKey: string | null,
): ExternalSkillSearchItem[] {
  return [...items]
    .filter((item) => !sourceKey || externalItemSourceKey(item) === sourceKey)
    .sort(compareExternalItems);
}
