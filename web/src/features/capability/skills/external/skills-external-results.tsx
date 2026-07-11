import { Download, Loader2, Puzzle } from "lucide-react";
import { useMemo, useState } from "react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import {
  externalSkillKey,
  getExternalSkillImportState,
} from "../controller/skill-marketplace-controller";
import { SkillStatePill } from "../catalog/skill-state-pill";
import {
  compareExternalItems,
  externalItemSourceKey,
  groupExternalResultsBySource,
  sourceGroupEmptyMessage,
  sourceGroupSummaryLabel,
} from "./external-results-model";
import { formatInstalls } from "./skills-helpers";

interface SkillsExternalResultsProps {
  busyExternalKeys: ReadonlySet<string>;
  importedExternalSources: Map<string, Set<string>>;
  loading: boolean;
  onImport: (item: ExternalSkillSearchItem) => void;
  onPreview: (item: ExternalSkillSearchItem) => void;
  results: ExternalSkillSearchItem[];
  sourceStatuses: ExternalSkillSourceStatus[];
  sources: ExternalSkillSourceInfo[];
  submittedQuery: string;
}

export function SkillsExternalResults({
  busyExternalKeys,
  importedExternalSources,
  loading,
  onImport,
  onPreview,
  results,
  sourceStatuses,
  sources,
  submittedQuery,
}: SkillsExternalResultsProps) {
  const { t } = useI18n();
  const [activeSourceKey, setActiveSourceKey] = useState<string | null>(null);
  const sourceGroups = useMemo(
    () => {
      if (!submittedQuery.trim() && !results.length) {
        return [];
      }
      return groupExternalResultsBySource(
        results,
        sourceStatuses,
        sources,
      );
    },
    [results, sourceStatuses, sources, submittedQuery],
  );
  const selectedSourceKey = sourceGroups.some((group) => group.key === activeSourceKey)
    ? activeSourceKey
    : null;
  const selectedSource = selectedSourceKey
    ? sourceGroups.find((group) => group.key === selectedSourceKey)
    : null;
  const visibleResults = useMemo(
    () => [...results]
      .filter((item) => !selectedSourceKey || externalItemSourceKey(item) === selectedSourceKey)
      .sort(compareExternalItems),
    [results, selectedSourceKey],
  );

  if (loading) {
    return (
      <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("capability.skills_external_loading")}
      </div>
    );
  }

  if (submittedQuery && !results.length && !sourceGroups.length) {
    return (
      <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-5 py-8 text-center text-sm text-(--text-soft)">
        {t("capability.skills_external_empty")}
      </div>
    );
  }

  if (!results.length && !sourceGroups.length) return null;

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
          <span className="shrink-0">{results.length} 个</span>
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
              busyExternalKeys={busyExternalKeys}
              importedExternalSources={importedExternalSources}
              item={item}
              onImport={() => onImport(item)}
              onPreview={() => onPreview(item)}
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

/* ── 外部结果行 ─────────────────────────────── */

interface ExternalResultRowProps {
  item: ExternalSkillSearchItem;
  busyExternalKeys: ReadonlySet<string>;
  importedExternalSources: Map<string, Set<string>>;
  onPreview: () => void;
  onImport: () => void;
}

function ExternalResultRow({
  item,
  busyExternalKeys,
  importedExternalSources,
  onPreview: onPreview,
  onImport: onImport,
}: ExternalResultRowProps) {
  const { alreadyImported, nameConflict: hasNameConflict } =
    getExternalSkillImportState(item, importedExternalSources);
  const isBusy = busyExternalKeys.has(externalSkillKey(item));
  const stateLabel = alreadyImported ? "已导入" : hasNameConflict ? "同名冲突" : "可导入";
  const stateTone = alreadyImported ? "success" : hasNameConflict ? "warning" : "neutral";
  const sourceLabel = item.source_name || item.source_kind || "社区";
  const sourceRef = item.package_spec || item.git_url || item.raw_url || item.source;

  return (
    <UiListRow
      className="min-h-[72px] rounded-[14px] px-2 py-1.5"
      leading={(
        <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_9%,var(--surface-panel-background))] text-sky-600 shadow-[0_1px_2px_rgba(15,23,42,0.04)]">
          <Puzzle className="h-4 w-4" />
        </span>
      )}
      onClick={onPreview}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <SkillStatePill tone={stateTone}>
            {stateLabel}
          </SkillStatePill>
          {!alreadyImported && !hasNameConflict ? (
            <UiListActionButton
              className="text-(--primary) hover:text-(--primary)"
              disabled={isBusy || hasNameConflict}
              onClick={onImport}
              size="sm"
              stopPropagation
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
          <span className="shrink-0">{formatInstalls(item.installs)} 次安装</span>
        </div>
      </div>
    </UiListRow>
  );
}
