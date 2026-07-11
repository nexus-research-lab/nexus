import { Clock3, Link2, Search } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiSearchInput } from "@/shared/ui/form/form-control";
import type { MemoryDocument, MemorySnapshot } from "@/types/memory/memory";

import { formatMemoryModifiedTime, memoryAgeDays } from "../memory-utils";
import {
  MEMORY_FILTER_OPTIONS,
  type MemoryFilter,
} from "./memory-catalog-model";
import { getMemoryDocumentPresentation } from "./memory-catalog-presentation";

interface AgentMemoryCatalogProps {
  filter: MemoryFilter;
  indexVisible: boolean;
  onFilterChange: (filter: MemoryFilter) => void;
  onQueryChange: (query: string) => void;
  onSelectDocument: (path: string) => void;
  query: string;
  selectedPath: string;
  snapshot: MemorySnapshot | null;
  visibleDocuments: MemoryDocument[];
}

export function AgentMemoryCatalog({
  filter,
  indexVisible,
  onFilterChange,
  onQueryChange,
  onSelectDocument,
  query,
  selectedPath,
  snapshot,
  visibleDocuments,
}: AgentMemoryCatalogProps) {
  const { locale, t } = useI18n();
  return (
    <aside className="nexus-memory-catalog flex min-h-0 min-w-0 flex-col border-r border-(--divider-subtle-color) bg-(--surface-raised-background)">
      <div className="shrink-0 border-b border-(--divider-subtle-color) p-3">
        <UiSearchInput
          className="w-full"
          inputClassName="text-[12px]"
          onChange={onQueryChange}
          placeholder={t("capability.memory_search_placeholder")}
          value={query}
        />
        <div className="soft-scrollbar mt-2.5 flex gap-1 overflow-x-auto" role="tablist">
          {MEMORY_FILTER_OPTIONS.map((option) => (
            <button
              aria-selected={filter === option.value}
              className={cn(
                "shrink-0 rounded-[6px] px-2 py-1 text-[10.5px] font-medium transition-colors",
                filter === option.value
                  ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                  : "text-(--text-soft) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
              )}
              key={option.value}
              onClick={() => onFilterChange(option.value)}
              role="tab"
              type="button"
            >
              {t(option.labelKey)}
            </button>
          ))}
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 py-2">
        {indexVisible && snapshot?.index ? (
          <div className="mb-2">
            <MemorySectionLabel label={t("capability.memory_index")} />
            <MemoryDocumentRow
              document={snapshot.index}
              isSelected={selectedPath === snapshot.index.path}
              locale={locale}
              onSelect={onSelectDocument}
            />
          </div>
        ) : null}

        {visibleDocuments.length > 0 ? (
          <div>
            <MemorySectionLabel
              label={t("capability.memory_documents")}
              value={String(visibleDocuments.length)}
            />
            <div className="space-y-0.5">
              {visibleDocuments.map((document) => (
                <MemoryDocumentRow
                  document={document}
                  isSelected={selectedPath === document.path}
                  key={document.path}
                  locale={locale}
                  onSelect={onSelectDocument}
                />
              ))}
            </div>
          </div>
        ) : !indexVisible ? (
          <div className="px-3 py-10 text-center">
            <Search className="mx-auto h-5 w-5 text-(--icon-muted)" />
            <p className="mt-2 text-[12px] text-(--text-muted)">
              {t("capability.memory_empty_filter")}
            </p>
          </div>
        ) : null}

        {snapshot?.truncated ? (
          <p className="px-3 py-3 text-[10.5px] leading-4 text-(--text-soft)">
            {t("capability.memory_truncated")}
          </p>
        ) : null}
      </div>

      {snapshot?.layout === "empty" ? (
        <div className="border-t border-(--divider-subtle-color) px-4 py-4">
          <p className="text-[12px] font-semibold text-(--text-strong)">
            {t("capability.memory_empty_title")}
          </p>
          <p className="mt-1 text-[11px] leading-5 text-(--text-muted)">
            {t("capability.memory_empty_description")}
          </p>
        </div>
      ) : null}
    </aside>
  );
}

function MemorySectionLabel({ label, value }: { label: string; value?: string }) {
  return (
    <div className="flex items-center justify-between px-2 pb-1 pt-1 text-[10px] font-semibold uppercase text-(--text-soft)">
      <span>{label}</span>
      {value ? <span className="tabular-nums">{value}</span> : null}
    </div>
  );
}

function MemoryDocumentRow({
  document,
  isSelected,
  locale,
  onSelect,
}: {
  document: MemoryDocument;
  isSelected: boolean;
  locale: string;
  onSelect: (path: string) => void;
}) {
  const { t } = useI18n();
  const presentation = getMemoryDocumentPresentation(document);
  const Icon = presentation.icon;
  const stale = memoryAgeDays(document.modified_at) > 1;
  return (
    <button
      className={cn(
        "group relative flex w-full items-start gap-2.5 rounded-[7px] px-2.5 py-2.5 text-left transition-colors",
        isSelected
          ? "bg-(--surface-interactive-active-background)"
          : "hover:bg-(--surface-interactive-hover-background)",
      )}
      onClick={() => onSelect(document.path)}
      type="button"
    >
      {isSelected ? (
        <span className="absolute bottom-2 left-0 top-2 w-[2px] rounded-full bg-(--primary)" />
      ) : null}
      <span className={cn(
        "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px]",
        presentation.tone,
      )}>
        <Icon className="h-3.5 w-3.5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[12px] font-semibold text-(--text-strong)">
            {document.title}
          </span>
          {document.indexed && document.kind === "topic" ? (
            <Link2 className="h-3 w-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
          ) : null}
        </span>
        <span className="mt-0.5 line-clamp-2 block text-[10.5px] leading-4 text-(--text-muted)">
          {document.description || document.path}
        </span>
        <span className="mt-1 flex items-center gap-1.5 text-[9.5px] text-(--text-soft)">
          <span>{t(presentation.labelKey)}</span>
          <span aria-hidden="true">·</span>
          <Clock3 className="h-2.5 w-2.5" />
          <span className={stale ? "text-amber-600 dark:text-amber-400" : undefined}>
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </span>
        </span>
      </span>
    </button>
  );
}
