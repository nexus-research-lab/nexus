import { Download, Loader2, Puzzle } from "lucide-react";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import { UiListActionButton } from "@/shared/ui/list-action";
import { UiListRow } from "@/shared/ui/list-row";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import { format_installs } from "./skills-helpers";
import { SkillStatePill } from "./skill-state-pill";
import type { SkillMarketplaceController } from "./skills-view-model";

interface SkillsExternalResultsProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsExternalResults({ ctrl }: SkillsExternalResultsProps) {
  const { t } = useI18n();
  const [activeSourceKey, setActiveSourceKey] = useState<string | null>(null);
  const sourceGroups = useMemo(
    () => {
      if (!ctrl.external_submitted_query.trim() && !ctrl.external_results.length) {
        return [];
      }
      return groupExternalResultsBySource(
        ctrl.external_results,
        ctrl.external_source_statuses,
        ctrl.external_sources,
      );
    },
    [ctrl.external_submitted_query, ctrl.external_results, ctrl.external_source_statuses, ctrl.external_sources],
  );
  const selectedSourceKey = sourceGroups.some((group) => group.key === activeSourceKey)
    ? activeSourceKey
    : null;
  const selectedSource = selectedSourceKey
    ? sourceGroups.find((group) => group.key === selectedSourceKey)
    : null;
  const visibleResults = useMemo(
    () => [...ctrl.external_results]
      .filter((item) => !selectedSourceKey || externalItemSourceKey(item) === selectedSourceKey)
      .sort(compareExternalItems),
    [ctrl.external_results, selectedSourceKey],
  );

  if (ctrl.external_loading) {
    return (
      <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("capability.skills_external_loading")}
      </div>
    );
  }

  if (ctrl.external_submitted_query && !ctrl.external_results.length && !sourceGroups.length) {
    return (
      <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-5 py-8 text-center text-sm text-(--text-soft)">
        {t("capability.skills_external_empty")}
      </div>
    );
  }

  if (!ctrl.external_results.length && !sourceGroups.length) return null;

  return (
    <section>
      <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
        <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
          {t("capability.search_results")}
        </h2>
        <span className="text-[12px] font-medium text-(--text-soft)">
          {t("capability.result_count", { count: visibleResults.length })}
        </span>
      </div>
      <div className="mb-4 flex flex-wrap gap-2">
        <button
          className={cn(
            "inline-flex max-w-full items-center gap-1.5 rounded-[8px] border px-2.5 py-1 text-left text-[11px] transition",
            !selectedSourceKey
              ? "border-(--primary) bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
              : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] text-(--text-muted) hover:border-(--primary)",
          )}
          onClick={() => setActiveSourceKey(null)}
          type="button"
        >
          <span className="truncate font-medium text-(--text-strong)">全部来源</span>
          <span className="shrink-0">{ctrl.external_results.length} 个</span>
        </button>
        {sourceGroups.map((group) => (
          <button
            key={group.key}
            className={cn(
              "inline-flex max-w-full items-center gap-1.5 rounded-[8px] border px-2.5 py-1 text-left text-[11px] transition",
              selectedSourceKey === group.key
                ? "border-(--primary) bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
                : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] text-(--text-muted) hover:border-(--primary)",
            )}
            onClick={() => setActiveSourceKey((current) => current === group.key ? null : group.key)}
            title={group.error || group.label}
            type="button"
          >
            <span className="truncate font-medium text-(--text-strong)">
              {group.label}
            </span>
            <span className="shrink-0">{sourceGroupSummaryLabel(group)}</span>
          </button>
        ))}
      </div>
      {visibleResults.length ? (
        <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
          {visibleResults.map((item: ExternalSkillSearchItem) => (
            <ExternalResultRow
              key={`${item.source_key || item.package_spec}@${item.skill_slug}`}
              busy_external_key={ctrl.busy_external_key}
              imported_external_sources={ctrl.imported_external_sources}
              item={item}
              on_import={() => void ctrl.handle_import_external(item)}
              on_preview={() => ctrl.handle_preview_external(item)}
            />
          ))}
        </div>
      ) : (
        <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-3 py-2 text-[12px] text-(--text-soft)">
          {selectedSource ? sourceGroupEmptyMessage(selectedSource) : t("capability.skills_external_empty")}
        </div>
      )}
    </section>
  );
}

interface ExternalResultGroup {
  key: string;
  label: string;
  kind: string;
  enabled: boolean;
  status: string;
  error?: string;
  items: ExternalSkillSearchItem[];
}

function groupExternalResultsBySource(
  items: ExternalSkillSearchItem[],
  statuses: ExternalSkillSourceStatus[],
  sources: ExternalSkillSourceInfo[],
): ExternalResultGroup[] {
  const groups = new Map<string, ExternalResultGroup>();
  const statusesByKey = new Map(statuses.map((status) => [status.key, status]));
  const sourceKeys = new Set<string>();

  [...sources].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name)).forEach((source) => {
    const status = statusesByKey.get(source.source_id);
    sourceKeys.add(source.source_id);
    groups.set(source.source_id, {
      key: source.source_id,
      label: source.name,
      kind: source.kind,
      enabled: source.enabled,
      status: source.enabled ? status?.status || "ok" : "disabled",
      error: status?.error || source.last_error,
      items: [],
    });
  });

  statuses.forEach((status) => {
    if (groups.has(status.key)) return;
    sourceKeys.add(status.key);
    groups.set(status.key, {
      key: status.key,
      label: status.name,
      kind: status.kind,
      enabled: true,
      status: status.status,
      error: status.error,
      items: [],
    });
  });

  for (const item of items) {
    const key = item.source_key || item.source_name || item.source_kind || "community";
    const existing = groups.get(key);
    if (existing) {
      existing.items.push(item);
      continue;
    }
    groups.set(key, {
      key,
      label: item.source_name || item.source_kind || "社区",
      kind: item.source_kind || "",
      enabled: true,
      status: "ok",
      items: [item],
    });
  }
  return [...groups.values()].filter((group) =>
    group.items.length > 0 ||
    sourceKeys.has(group.key) ||
    group.status === "error" ||
    group.status === "disabled"
  );
}

function sourceGroupEmptyMessage(group: ExternalResultGroup): string {
  if (group.status === "disabled") {
    return "该来源已停用，可在来源面板启用后参与搜索。";
  }
  if (group.status === "error") {
    return group.error ? `搜索失败：${group.error}` : "该来源搜索失败。";
  }
  return "该来源没有匹配结果。";
}

function sourceGroupSummaryLabel(group: ExternalResultGroup): string {
  if (group.status === "disabled") {
    return "已停用";
  }
  if (group.status === "error") {
    return "失败";
  }
  return `${group.items.length} 个`;
}

function externalItemSourceKey(item: ExternalSkillSearchItem): string {
  return item.source_key || item.source_name || item.source_kind || "community";
}

function compareExternalItems(a: ExternalSkillSearchItem, b: ExternalSkillSearchItem): number {
  if (a.installs !== b.installs) {
    return b.installs - a.installs;
  }
  const sourceCompare = (a.source_name || "").localeCompare(b.source_name || "");
  if (sourceCompare !== 0) {
    return sourceCompare;
  }
  return (a.title || a.name).localeCompare(b.title || b.name);
}

/* ── 外部结果行 ─────────────────────────────── */

interface ExternalResultRowProps {
  item: ExternalSkillSearchItem;
  busy_external_key: string | null;
  imported_external_sources: Map<string, Set<string>>;
  on_preview: () => void;
  on_import: () => void;
}

function ExternalResultRow({
  item,
  busy_external_key: busyExternalKey,
  imported_external_sources: importedExternalSources,
  on_preview: onPreview,
  on_import: onImport,
}: ExternalResultRowProps) {
  const importedSources = importedExternalSources.get(item.skill_slug);
  const alreadyImported = importedSources?.has(item.package_spec) ?? false;
  const hasNameConflict = !!importedSources && !alreadyImported;
  const externalKey = `${item.source_key || item.package_spec}@@${item.skill_slug}`;
  const isBusy = busyExternalKey === externalKey;
  const stateLabel = alreadyImported ? "已导入" : hasNameConflict ? "同名冲突" : "可导入";
  const stateTone = alreadyImported ? "success" : hasNameConflict ? "warning" : "neutral";
  const sourceLabel = item.source_name || item.source_kind || "社区";
  const sourceRef = item.package_spec || item.git_url || item.raw_url || item.source;

  return (
    <UiListRow
      class_name="min-h-[72px] rounded-[14px] px-2 py-1.5"
      leading={(
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_9%,var(--surface-panel-background))] text-sky-600 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <Puzzle className="h-4 w-4" />
        </span>
      )}
      on_click={onPreview}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <SkillStatePill tone={stateTone}>
            {stateLabel}
          </SkillStatePill>
          {!alreadyImported && !hasNameConflict ? (
            <UiListActionButton
              class_name="text-(--primary) hover:text-(--primary)"
              disabled={isBusy || hasNameConflict}
              onClick={onImport}
              size="sm"
              stop_propagation
              title="导入到技能库"
              visibility="visible"
            >
              {isBusy ? (
                <Loader2 className="h-3 w-3 animate-spin" />
              ) : (
                <Download className="h-3 w-3" />
              )}
            </UiListActionButton>
          ) : null}
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {item.title || item.skill_slug}
          </span>
          <UiBadge size="xs">{sourceLabel}</UiBadge>
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {item.description || item.readme_markdown || "暂无描述"}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-(--text-soft)">
          <span className="truncate">{sourceRef}</span>
          <span className="shrink-0">·</span>
          <span className="shrink-0">{format_installs(item.installs)} 次安装</span>
        </div>
      </div>
    </UiListRow>
  );
}
