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
    <div className="nexus-memory-document-content pb-5 pt-2">
      <div className="space-y-1">
        {entries.map((entry) => (
          <button
            className="group block w-full rounded-[8px] px-2 py-2.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background)"
            key={`${entry.path}:${entry.title}`}
            onClick={() => onSelectPath(entry.path)}
            type="button"
          >
            <span className="block min-w-0">
              <span className="block text-sm font-semibold text-(--text-strong)">
                {entry.title}
              </span>
              {entry.description ? (
                <span className="mt-0.5 line-clamp-1 block text-compact leading-5 text-(--text-muted)">
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
