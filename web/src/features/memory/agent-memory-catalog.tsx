import {
  BookOpenText,
  Clock3,
  FileText,
  FolderKanban,
  History,
  Link2,
  MessageSquareWarning,
  Search,
  UserRound,
  type LucideIcon,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiSearchInput } from "@/shared/ui/form-control";
import type {
  MemoryDocument,
  MemoryDocumentKind,
  MemoryDocumentType,
  MemorySnapshot,
} from "@/types/memory/memory";

import {
  formatMemoryModifiedTime,
  memoryAgeDays,
  type MemoryFilter,
} from "./memory-utils";

const FILTERS: MemoryFilter[] = [
  "all",
  "user",
  "feedback",
  "project",
  "reference",
  "daily_log",
];

const FILTER_LABEL_KEY: Readonly<Partial<Record<MemoryFilter, TranslationKey>>> = {
  user: "capability.memory_type_user",
  feedback: "capability.memory_type_feedback",
  project: "capability.memory_type_project",
  reference: "capability.memory_type_reference",
  daily_log: "capability.memory_type_daily_log",
};

const ICON_BY_KIND: Readonly<Partial<Record<MemoryDocumentKind, LucideIcon>>> = {
  index: BookOpenText,
  daily_log: History,
};

const ICON_BY_TYPE: Readonly<Partial<Record<MemoryDocumentType, LucideIcon>>> = {
  user: UserRound,
  feedback: MessageSquareWarning,
  project: FolderKanban,
  reference: Link2,
};

const TONE_BY_KIND: Readonly<Partial<Record<MemoryDocumentKind, string>>> = {
  index: "bg-sky-500/10 text-sky-600 dark:text-sky-400",
  daily_log: "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400",
};

const TONE_BY_TYPE: Readonly<Partial<Record<MemoryDocumentType, string>>> = {
  user: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400",
  feedback: "bg-amber-500/10 text-amber-700 dark:text-amber-400",
  project: "bg-blue-500/10 text-blue-600 dark:text-blue-400",
  reference: "bg-rose-500/10 text-rose-600 dark:text-rose-400",
};

const TYPE_LABEL_KEY: Readonly<Partial<Record<MemoryDocumentType, TranslationKey>>> = {
  user: "capability.memory_type_user",
  feedback: "capability.memory_type_feedback",
  project: "capability.memory_type_project",
  reference: "capability.memory_type_reference",
};

export interface AgentMemoryCatalogModel {
  filter: MemoryFilter;
  indexVisible: boolean;
  locale: string;
  query: string;
  selectedPath: string;
  snapshot: MemorySnapshot | null;
  visibleDocuments: MemoryDocument[];
}

interface AgentMemoryCatalogProps {
  model: AgentMemoryCatalogModel;
  onFilterChange: (filter: MemoryFilter) => void;
  onQueryChange: (query: string) => void;
  onSelectDocument: (path: string) => void;
}

export function AgentMemoryCatalog({
  model,
  onFilterChange,
  onQueryChange,
  onSelectDocument,
}: AgentMemoryCatalogProps) {
  const { t } = useI18n();
  return (
    <aside className="nexus-memory-catalog flex min-h-0 min-w-0 flex-col border-r border-(--divider-subtle-color) bg-(--surface-raised-background)">
      <div className="shrink-0 border-b border-(--divider-subtle-color) p-3">
        <UiSearchInput
          className="w-full"
          inputClassName="text-[12px]"
          onChange={onQueryChange}
          placeholder={t("capability.memory_search_placeholder")}
          value={model.query}
        />
        <div className="soft-scrollbar mt-2.5 flex gap-1 overflow-x-auto" role="tablist">
          {FILTERS.map((filter) => (
            <button
              aria-selected={model.filter === filter}
              className={cn(
                "shrink-0 rounded-[6px] px-2 py-1 text-[10.5px] font-medium transition-colors",
                model.filter === filter
                  ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                  : "text-(--text-soft) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
              )}
              key={filter}
              onClick={() => onFilterChange(filter)}
              role="tab"
              type="button"
            >
              {t(FILTER_LABEL_KEY[filter] ?? "capability.memory_filter_all")}
            </button>
          ))}
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 py-2">
        {model.indexVisible && model.snapshot?.index ? (
          <div className="mb-2">
            <MemorySectionLabel label={t("capability.memory_index")} />
            <MemoryDocumentRow
              document={model.snapshot.index}
              isSelected={model.selectedPath === model.snapshot.index.path}
              locale={model.locale}
              onSelect={onSelectDocument}
              typeLabel={t("capability.memory_index")}
            />
          </div>
        ) : null}

        {model.visibleDocuments.length > 0 ? (
          <div>
            <MemorySectionLabel
              label={t("capability.memory_documents")}
              value={String(model.visibleDocuments.length)}
            />
            <div className="space-y-0.5">
              {model.visibleDocuments.map((document) => (
                <MemoryDocumentRow
                  document={document}
                  isSelected={model.selectedPath === document.path}
                  key={document.path}
                  locale={model.locale}
                  onSelect={onSelectDocument}
                  typeLabel={document.kind === "daily_log"
                    ? t("capability.memory_type_daily_log")
                    : t(TYPE_LABEL_KEY[document.type ?? ""] ?? "capability.memory_type_topic")}
                />
              ))}
            </div>
          </div>
        ) : !model.indexVisible ? (
          <div className="px-3 py-10 text-center">
            <Search className="mx-auto h-5 w-5 text-(--icon-muted)" />
            <p className="mt-2 text-[12px] text-(--text-muted)">
              {t("capability.memory_empty_filter")}
            </p>
          </div>
        ) : null}

        {model.snapshot?.truncated ? (
          <p className="px-3 py-3 text-[10.5px] leading-4 text-(--text-soft)">
            {t("capability.memory_truncated")}
          </p>
        ) : null}
      </div>

      {model.snapshot?.layout === "empty" ? (
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
  typeLabel,
}: {
  document: MemoryDocument;
  isSelected: boolean;
  locale: string;
  onSelect: (path: string) => void;
  typeLabel: string;
}) {
  const Icon = ICON_BY_KIND[document.kind] ?? ICON_BY_TYPE[document.type ?? ""] ?? FileText;
  const tone = TONE_BY_KIND[document.kind]
    ?? TONE_BY_TYPE[document.type ?? ""]
    ?? "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
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
        tone,
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
          <span>{typeLabel}</span>
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
