import { Loader2 } from "lucide-react";
import { useMemo, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import { ExternalResultRow } from "./external-result-row";
import {
  buildExternalResultsModel,
  sourceGroupEmptyMessage,
  sourceGroupSummaryLabel,
  type ExternalResultGroup,
  type ExternalResultsModel,
} from "./external-results-model";
import { externalSkillKey } from "./external-skill-model";

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
  const [activeSourceKey, setActiveSourceKey] = useState<string | null>(null);
  const model = useMemo(
    () => buildExternalResultsModel({
      activeSourceKey,
      items: results,
      loading,
      sources,
      statuses: sourceStatuses,
      submittedQuery,
    }),
    [activeSourceKey, loading, results, sourceStatuses, sources, submittedQuery],
  );

  return (
    <ExternalResultsStage
      busyExternalKeys={busyExternalKeys}
      importedExternalSources={importedExternalSources}
      model={model}
      onImport={onImport}
      onPreview={onPreview}
      onSelectSource={setActiveSourceKey}
      totalCount={results.length}
    />
  );
}

interface ExternalResultsStageProps {
  busyExternalKeys: ReadonlySet<string>;
  importedExternalSources: Map<string, Set<string>>;
  model: ExternalResultsModel;
  onImport: (item: ExternalSkillSearchItem) => void;
  onPreview: (item: ExternalSkillSearchItem) => void;
  onSelectSource: (key: string | null) => void;
  totalCount: number;
}

function ExternalResultsStage(props: ExternalResultsStageProps) {
  const { t } = useI18n();
  if (props.model.phase === "hidden") return null;
  if (props.model.phase === "loading") {
    return (
      <div className="flex items-center justify-center gap-2 py-12 text-sm text-(--text-soft)">
        <Loader2 className="h-4 w-4 animate-spin" />
        {t("capability.skills_external_loading")}
      </div>
    );
  }
  if (props.model.phase === "empty") {
    return (
      <div className="rounded-[12px] border border-dashed border-(--divider-subtle-color) px-5 py-8 text-center text-sm text-(--text-soft)">
        {t("capability.skills_external_empty")}
      </div>
    );
  }
  return <ExternalResultsReady {...props} />;
}

function ExternalResultsReady({
  busyExternalKeys,
  importedExternalSources,
  model,
  onImport,
  onPreview,
  onSelectSource,
  totalCount,
}: ExternalResultsStageProps) {
  const { t } = useI18n();
  return (
    <section>
      <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
        <h2 className="text-[18px] font-medium text-(--text-strong)">
          {t("capability.search_results")}
        </h2>
        <span className="text-[12px] font-medium text-(--text-soft)">
          {t("capability.result_count", { count: model.visibleItems.length })}
        </span>
      </div>
      <ExternalSourceFilters
        groups={model.groups}
        onSelect={onSelectSource}
        selectedSourceKey={model.selectedSourceKey}
        totalCount={totalCount}
      />
      {model.visibleItems.length ? (
        <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
          {model.visibleItems.map((item) => (
            <ExternalResultRow
              key={externalSkillKey(item)}
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
          {model.selectedGroup
            ? sourceGroupEmptyMessage(model.selectedGroup)
            : t("capability.skills_external_empty")}
        </div>
      )}
    </section>
  );
}

interface ExternalSourceFiltersProps {
  groups: ExternalResultGroup[];
  onSelect: (key: string | null) => void;
  selectedSourceKey: string | null;
  totalCount: number;
}

function ExternalSourceFilters({
  groups,
  onSelect,
  selectedSourceKey,
  totalCount,
}: ExternalSourceFiltersProps) {
  return (
    <div className="mb-4 flex flex-wrap gap-2">
      <ExternalSourceFilter
        label="全部来源"
        onClick={() => onSelect(null)}
        selected={!selectedSourceKey}
        summary={`${totalCount} 个`}
      />
      {groups.map((group) => (
        <ExternalSourceFilter
          key={group.key}
          label={group.label}
          onClick={() => onSelect(selectedSourceKey === group.key ? null : group.key)}
          selected={selectedSourceKey === group.key}
          summary={sourceGroupSummaryLabel(group)}
          title={group.error || group.label}
        />
      ))}
    </div>
  );
}

interface ExternalSourceFilterProps {
  label: string;
  onClick: () => void;
  selected: boolean;
  summary: string;
  title?: string;
}

function ExternalSourceFilter({
  label,
  onClick,
  selected,
  summary,
  title,
}: ExternalSourceFilterProps) {
  return (
    <button
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-[8px] border px-2.5 py-1 text-left text-[11px] transition",
        selected
          ? "border-(--primary) bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
          : "border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] text-(--text-muted) hover:border-(--primary)",
      )}
      onClick={onClick}
      title={title}
      type="button"
    >
      <span className="truncate font-medium text-(--text-strong)">{label}</span>
      <span className="shrink-0">{summary}</span>
    </button>
  );
}
