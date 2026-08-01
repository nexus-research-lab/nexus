import { Link2 } from "lucide-react";

import type { MemoryIndexEntry } from "./memory-index-model";

interface MemoryIndexEntriesProps {
  entries: MemoryIndexEntry[];
  onSelectPath: (path: string) => void;
}

export function MemoryIndexEntries({
  entries,
  onSelectPath,
}: MemoryIndexEntriesProps) {
  return (
    <div className="mx-auto w-full max-w-[860px] px-5 pb-5 pt-2">
      <div className="space-y-1">
        {entries.map((entry) => (
          <button
            className="group flex w-full items-start gap-3 rounded-[8px] px-2 py-2.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background)"
            key={`${entry.path}:${entry.title}`}
            onClick={() => onSelectPath(entry.path)}
            type="button"
          >
            <Link2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted) group-hover:text-(--primary)" />
            <span className="min-w-0 flex-1">
              <span className="block text-sm font-semibold text-(--text-strong)">
                {entry.title}
              </span>
              {entry.description ? (
                <span className="mt-0.5 line-clamp-2 block text-compact leading-5 text-(--text-muted)">
                  {entry.description}
                </span>
              ) : null}
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
