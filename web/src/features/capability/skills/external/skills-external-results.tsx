import { Loader2 } from "lucide-react";
import { useMemo } from "react";

import { CapabilitySectionHeader } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { WORKSPACE_CATALOG_GRID_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
} from "@/types/capability/skill";

import { ExternalResultCard } from "./external-result-card";
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
  onSelectSource: (key: string | null) => void;
  results: ExternalSkillSearchItem[];
  sourceStatuses: ExternalSkillSourceStatus[];
  selectedSourceKey: string | null;
  sources: ExternalSkillSourceInfo[];
  submittedQuery: string;
}

export function SkillsExternalResults({
  busyExternalKeys,
  importedExternalSources,
  loading,
  onImport,
  onPreview,
  onSelectSource,
  results,
  sourceStatuses,
  selectedSourceKey,
  sources,
  submittedQuery,
}: SkillsExternalResultsProps) {
  const { t } = useI18n();
  const model = useMemo(
    () => buildExternalResultsModel({
      activeSourceKey: selectedSourceKey,
      items: results,
      localization: { t },
      loading,
      sources,
      statuses: sourceStatuses,
      submittedQuery,
    }),
    [loading, results, selectedSourceKey, sourceStatuses, sources, submittedQuery, t],
  );

  return (
    <ExternalResultsStage
      busyExternalKeys={busyExternalKeys}
      importedExternalSources={importedExternalSources}
      model={model}
      onImport={onImport}
      onPreview={onPreview}
      onSelectSource={onSelectSource}
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
      <div className="rounded-[8px] border border-dashed border-(--divider-subtle-color) px-4 py-6 text-center text-compact text-(--text-soft)">
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
      <CapabilitySectionHeader
        count={t("capability.result_count", {
          count: model.visibleItems.length,
        })}
        title={model.title}
      />
      {model.showSourceFilters ? (
        <ExternalSourceFilters
          groups={model.groups}
          onSelect={onSelectSource}
          selectedSourceKey={model.selectedSourceKey}
          totalCount={totalCount}
        />
      ) : null}
      {model.visibleItems.length ? (
        <div className={`${WORKSPACE_CATALOG_GRID_CLASS_NAME} gap-2.5`}>
          {model.visibleItems.map((item) => (
            <ExternalResultCard
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
        <div className="rounded-[8px] border border-dashed border-(--divider-subtle-color) px-3 py-2 text-xs text-(--text-soft)">
          {model.selectedGroup
            ? sourceGroupEmptyMessage(model.selectedGroup, { t })
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
  const { t } = useI18n();
  return (
    <div className="mb-3 flex flex-wrap gap-1.5">
      <ExternalSourceFilter
        label={t("capability.skills_external_all_sources")}
        onClick={() => onSelect(null)}
        selected={!selectedSourceKey}
        summary={t("capability.skills_external_source_result_count", {
          count: totalCount,
        })}
      />
      {groups.map((group) => (
        <ExternalSourceFilter
          key={group.key}
          label={group.label}
          onClick={() => onSelect(selectedSourceKey === group.key ? null : group.key)}
          disabled={group.status === "disabled"}
          selected={selectedSourceKey === group.key}
          summary={sourceGroupSummaryLabel(group, { t })}
          title={group.error || group.label}
        />
      ))}
    </div>
  );
}

interface ExternalSourceFilterProps {
  disabled?: boolean;
  label: string;
  onClick: () => void;
  selected: boolean;
  summary: string;
  title?: string;
}

function ExternalSourceFilter({
  disabled = false,
  label,
  onClick,
  selected,
  summary,
  title,
}: ExternalSourceFilterProps) {
  return (
    <button
      className={cn(
        "inline-flex max-w-full items-center gap-1.5 rounded-[6px] border px-2 py-0.5 text-left text-2xs transition",
        disabled && "cursor-not-allowed opacity-50",
        selected
          ? "border-(--primary) bg-[color:color-mix(in_srgb,var(--primary)_12%,transparent)] text-(--primary)"
          : "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:border-(--primary)",
      )}
      disabled={disabled}
      onClick={onClick}
      title={title}
      type="button"
    >
      <span className="truncate font-medium text-(--text-strong)">{label}</span>
      <span className="shrink-0">{summary}</span>
    </button>
  );
}
