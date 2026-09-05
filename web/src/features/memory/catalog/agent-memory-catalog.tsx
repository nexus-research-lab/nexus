/**
 * INPUT: 已投影的记忆分区、筛选、查询和目录动作。
 * OUTPUT: 紧凑记忆目录，以可读摘要为主标题并保留可区分的文档名。
 * POS: Agent 记忆页左栏，不读取正文或解释路径协议。
 */
import { RefreshCw, Search } from "lucide-react";

import { UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import { UiListRow } from "@/shared/ui/list/list-row";
import { UiSelectMenu } from "@/shared/ui/menu/select-menu";
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
  const filterOptions = MEMORY_FILTER_OPTIONS.map((option) => ({
    label: t(option.labelKey),
    value: option.value,
  }));
  return (
    <aside className="nexus-memory-catalog flex min-h-0 min-w-0 flex-col bg-(--surface-raised-background)">
      <div className="flex shrink-0 items-center gap-2 px-3 py-3">
        <UiSearchInput
          action={(
            <UiIconButton
              aria-label={t("capability.refresh")}
              className="-mr-1 shrink-0"
              disabled={refreshing}
              onClick={onRefresh}
              size="xs"
              title={t("capability.refresh")}
              variant="ghost"
            >
              <RefreshCw
                className={refreshing
                  ? getUiSpinnerClassName({ size: "sm" })
                  : "h-3.5 w-3.5"}
              />
            </UiIconButton>
          )}
          className="min-w-0 flex-1"
          onChange={onQueryChange}
          placeholder={t("capability.memory_search_placeholder")}
          value={query}
        />
        <UiSelectMenu
          ariaLabel={t("capability.memory_filter_aria")}
          className="w-[86px] shrink-0"
          menuMinWidth={120}
          onChange={(value) => onFilterChange(value as MemoryFilter)}
          options={filterOptions}
          placement="bottom"
          size="sm"
          value={filter}
        />
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 pb-3">
        {sections.map((section) => (
          <MemoryCatalogSectionView
            key={section.key}
            onSelect={onSelectDocument}
            section={section}
          />
        ))}
        {emptyFilterVisible ? (
          <div className="px-3 py-10 text-center">
            <Search className="mx-auto h-5 w-5 text-(--icon-muted)" />
            <p className="mt-2 text-compact text-(--text-muted)">
              {t("capability.memory_empty_filter")}
            </p>
          </div>
        ) : null}

        {truncated ? (
          <p className="px-3 py-3 text-xs leading-4 text-(--text-soft)">
            {t("capability.memory_truncated")}
          </p>
        ) : null}
      </div>

      {emptyMemoryVisible ? (
        <div className="px-4 py-4">
          <p className="text-compact font-semibold text-(--text-strong)">
            {t("capability.memory_empty_title")}
          </p>
          <p className="mt-1 text-xs leading-5 text-(--text-muted)">
            {t("capability.memory_empty_description")}
          </p>
        </div>
      ) : null}
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
    <div className="flex items-center justify-between px-2 pb-1 pt-1 text-2xs font-semibold uppercase text-(--text-soft)">
      <span>{label}</span>
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
