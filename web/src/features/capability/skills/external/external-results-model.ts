import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

export interface ExternalResultGroup {
  error?: string;
  items: ExternalSkillSearchItem[];
  key: string;
  label: string;
  status: string;
}

export function groupExternalResultsBySource(
  items: ExternalSkillSearchItem[],
  statuses: ExternalSkillSourceStatus[],
  sources: ExternalSkillSourceInfo[],
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
        error: status?.error || source.last_error,
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
      error: status.error,
      items: [],
      key: status.key,
      label: status.name,
      status: status.status,
    });
  });

  items.forEach((item) => {
    const key = externalItemSourceKey(item);
    const group = groups.get(key) ?? createResultGroup(item, key);
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
): ExternalResultGroup {
  return {
    items: [],
    key,
    label: item.source_name || item.source_kind || "社区",
    status: "ok",
  };
}

export function sourceGroupEmptyMessage(group: ExternalResultGroup): string {
  const messages: Record<string, string> = {
    disabled: "该来源已停用，可在来源面板启用后参与搜索。",
    error: group.error ? `搜索失败：${group.error}` : "该来源搜索失败。",
    ok: "该来源没有匹配结果。",
  };
  return messages[group.status] ?? messages.ok;
}

export function sourceGroupSummaryLabel(group: ExternalResultGroup): string {
  const labels: Record<string, string> = {
    disabled: "已停用",
    error: "失败",
    ok: `${group.items.length} 个`,
  };
  return labels[group.status] ?? labels.ok;
}

export function externalItemSourceKey(item: ExternalSkillSearchItem): string {
  return item.source_key || item.source_name || item.source_kind || "community";
}

export function compareExternalItems(
  left: ExternalSkillSearchItem,
  right: ExternalSkillSearchItem,
): number {
  if (left.installs !== right.installs) return right.installs - left.installs;
  const sourceOrder = (left.source_name || "").localeCompare(right.source_name || "");
  return sourceOrder || (left.title || left.name).localeCompare(right.title || right.name);
}
