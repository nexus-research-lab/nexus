/**
 * INPUT: 已投影的记忆分区、筛选、查询和目录动作。
 * OUTPUT: 共享搜索/类型筛选、互斥空状态与紧凑记忆目录，保留可读摘要和文档名。
 * POS: Agent 记忆页左栏，不读取正文或解释路径协议。
 */
import { RefreshCw, Search } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { SidebarSearchAction, SidebarSearchField } from "@/shared/ui/form/sidebar-search-field";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiFilterSelect } from "@/shared/ui/menu/filter-select";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import {
  MEMORY_FILTER_OPTIONS,
  type MemoryCatalogRow,
  type MemoryCatalogSection,
  type MemoryFilter,
} from "./memory-catalog-model";
import {
  getMemoryDocumentDisplayTitle,
  getMemoryDocumentIcon,
} from "./memory-catalog-presentation";

interface AgentMemoryCatalogProps {
  emptyFilterVisible: boolean;
  emptyMemoryVisible: boolean;
  filter: MemoryFilter;
  onFilterChange: (filter: MemoryFilter) => void;
  onQueryChange: (query: string) => void;
  onRefresh: () => void;
  onSelectDocument: (path: string) => void;
  query: string;
  refreshing: boolean;
  sections: MemoryCatalogSection[];
  truncated: boolean;
}

export function AgentMemoryCatalog({
  emptyFilterVisible,
  emptyMemoryVisible,
  filter,
  onFilterChange,
  onQueryChange,
  onRefresh,
  onSelectDocument,
  query,
  refreshing,
  sections,
  truncated,
}: AgentMemoryCatalogProps) {
  const { t } = useI18n();
  const hasFilters = filter !== "all" || !createUiSearchMatcher(query).empty;
  const filterOptions = MEMORY_FILTER_OPTIONS.map((option) => ({
    label: t(option.labelKey),
    value: option.value,
  }));
  return (
    <aside className="nexus-memory-catalog flex min-h-0 min-w-0 flex-col bg-(--surface-shell-directory-background)">
      <div className="shrink-0 space-y-1 pt-3">
        <SidebarSearchField
          action={(
            <SidebarSearchAction
              aria-label={t("capability.refresh")}
              aria-busy={refreshing || undefined}
              disabled={refreshing}
              onClick={onRefresh}
              title={t("capability.refresh")}
            >
              <RefreshCw
                className={refreshing
                  ? getUiSpinnerClassName({ size: "sm" })
                  : undefined}
              />
            </SidebarSearchAction>
          )}
          label={t("capability.memory_search_placeholder")}
          onChange={onQueryChange}
          value={query}
        />
        <div className="px-2.5 pb-2 max-[559px]:px-4">
          <UiFilterSelect
            ariaLabel={t("capability.memory_filter_aria")}
            className="w-full sm:w-full"
            label={t("capability.memory_filter_label")}
            onChange={(value) => onFilterChange(value as MemoryFilter)}
            options={filterOptions}
            value={filter}
          />
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {sections.map((section) => (
          <MemoryCatalogSectionView
            key={section.key}
            onSelect={onSelectDocument}
            section={section}
          />
        ))}
        {emptyMemoryVisible ? (
          <UiResourceState
            description={t("capability.memory_empty_description")}
            size="sm" state="empty" variant="plain"
            title={t("capability.memory_empty_title")}
          />
        ) : emptyFilterVisible ? (
          <UiResourceState
            icon={<Search aria-hidden className="h-5 w-5 text-(--icon-default)" />}
            primaryAction={hasFilters ? {
              label: t("capability.memory_clear_filters"),
              onClick: () => { onQueryChange(""); onFilterChange("all"); },
            } : undefined}
            size="sm" state="empty" variant="plain"
            title={t("capability.memory_empty_filter")}
          />
        ) : null}

        {truncated ? (
          <p className={cn("px-3 py-3", getUiTypographyClassName({ role: "supporting", tone: "muted" }))}>
            {t("capability.memory_truncated")}
          </p>
        ) : null}
      </div>

    </aside>
  );
}

function MemoryCatalogSectionView({
  onSelect,
  section,
}: {
  onSelect: (path: string) => void;
  section: MemoryCatalogSection;
}) {
  const { t } = useI18n();
  return (
    <div className={section.key === "index" ? "mb-2" : undefined}>
      {section.countVisible ? (
        <MemorySectionLabel
          label={t(section.labelKey)}
          value={String(section.rows.length)}
        />
      ) : null}
      <div className="space-y-0.5">
        {section.rows.map((row) => (
          <MemoryDocumentRow
            key={row.document.path}
            onSelect={onSelect}
            row={row}
          />
        ))}
      </div>
    </div>
  );
}

function MemorySectionLabel({ label, value }: { label: string; value?: string }) {
  return (
    <div className={cn("flex items-center justify-between gap-2 px-2 py-1.5", getUiTypographyClassName({ role: "metadata", tone: "muted", weight: "medium" }))}>
      <h3>{label}</h3>
      {value ? <span className="tabular-nums">{value}</span> : null}
    </div>
  );
}

function MemoryDocumentRow({
  onSelect,
  row,
}: {
  onSelect: (path: string) => void;
  row: MemoryCatalogRow;
}) {
  const { document, isSelected } = row;
  const Icon = getMemoryDocumentIcon(document);
  const displayTitle = getMemoryDocumentDisplayTitle(document);
  const showDocumentTitle = displayTitle !== document.title;
  return (
    <UiListRow
      active={isSelected}
      activeTone="sidebar"
      aria-pressed={isSelected}
      density="dense"
      description={showDocumentTitle ? document.title : undefined}
      leading={(
        <span className="flex h-6 w-6 shrink-0 items-center justify-center radius-control-xs bg-(--surface-panel-subtle-background) text-(--icon-muted)">
          <Icon className="h-3.5 w-3.5" />
        </span>
      )}
      onClick={() => onSelect(document.path)}
      title={displayTitle}
      tooltip={document.path}
    />
  );
}
