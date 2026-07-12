import { Check, Link2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

import type { MemoryIndexEntry } from "./memory-index-model";

interface MemoryIndexEntriesProps {
  entries: MemoryIndexEntry[];
  onSelectPath: (path: string) => void;
}

export function MemoryIndexEntries({
  entries,
  onSelectPath,
}: MemoryIndexEntriesProps) {
  const { t } = useI18n();
  return (
    <div className="mx-auto w-full max-w-[860px] px-5 py-5">
      <div className="mb-3 flex items-center gap-2 text-[12px] font-semibold text-(--text-muted)">
        <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
        {t("capability.memory_index_entries", { count: entries.length })}
      </div>
      <div className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
        {entries.map((entry) => (
          <button
            className="group flex w-full items-start gap-3 px-1 py-3.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background)"
            key={`${entry.path}:${entry.title}`}
            onClick={() => onSelectPath(entry.path)}
            type="button"
          >
            <Link2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted) group-hover:text-(--primary)" />
            <span className="min-w-0 flex-1">
              <span className="block text-[13px] font-semibold text-(--text-strong)">
                {entry.title}
              </span>
              {entry.description ? (
                <span className="mt-0.5 block text-[12px] leading-5 text-(--text-muted)">
                  {entry.description}
                </span>
              ) : null}
              <span className="mt-1 block truncate font-mono text-[10.5px] text-(--text-soft)">
                {entry.path}
              </span>
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
