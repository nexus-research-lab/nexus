import { UiListRow } from "@/shared/ui/list/list-row";

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
          <UiListRow
            density="dense"
            description={entry.description}
            key={`${entry.path}:${entry.title}`}
            onClick={() => onSelectPath(entry.path)}
            title={entry.title}
          />
        ))}
      </div>
    </div>
  );
}
