import type { TranslationKey } from "@/shared/i18n/messages";
import type { MemoryDocument, MemorySnapshot } from "@/types/memory/memory";

export type MemoryFilter = "all" | "user" | "feedback" | "project" | "reference" | "daily_log";

export interface MemoryFilterOption {
  labelKey: TranslationKey;
  value: MemoryFilter;
}

export const MEMORY_FILTER_OPTIONS: readonly MemoryFilterOption[] = [
  { labelKey: "capability.memory_filter_all", value: "all" },
  { labelKey: "capability.memory_type_user", value: "user" },
  { labelKey: "capability.memory_type_feedback", value: "feedback" },
  { labelKey: "capability.memory_type_project", value: "project" },
  { labelKey: "capability.memory_type_reference", value: "reference" },
  { labelKey: "capability.memory_type_daily_log", value: "daily_log" },
];

export interface MemoryCatalogProjection {
  allDocuments: MemoryDocument[];
  counts: {
    logs: number;
    topics: number;
  };
  indexVisible: boolean;
  latestDocument: MemoryDocument | null;
  selectedDocument: MemoryDocument | null;
  visibleDocuments: MemoryDocument[];
}

export function projectMemoryCatalog(
  snapshot: MemorySnapshot | null,
  selectedPath: string,
  filter: MemoryFilter,
  query: string,
): MemoryCatalogProjection {
  const allDocuments = getAllMemoryDocuments(snapshot);
  const documents = snapshot?.documents ?? [];
  return {
    allDocuments,
    counts: countMemoryDocuments(documents),
    indexVisible: Boolean(
      snapshot?.index && memoryDocumentMatches(snapshot.index, "index", query),
    ),
    latestDocument: documents[0] ?? snapshot?.index ?? null,
    selectedDocument: allDocuments.find((document) => document.path === selectedPath) ?? null,
    visibleDocuments: documents.filter((document) =>
      memoryDocumentMatches(document, filter, query)),
  };
}

export function resolveSelectedMemoryPath(
  snapshot: MemorySnapshot,
  currentPath: string,
): string {
  const documents = getAllMemoryDocuments(snapshot);
  return documents.some((document) => document.path === currentPath)
    ? currentPath
    : documents[0]?.path ?? "";
}

function getAllMemoryDocuments(snapshot: MemorySnapshot | null): MemoryDocument[] {
  return snapshot
    ? [snapshot.index, ...snapshot.documents].filter(Boolean) as MemoryDocument[]
    : [];
}

function countMemoryDocuments(documents: MemoryDocument[]): {
  logs: number;
  topics: number;
} {
  return documents.reduce(
    (counts, document) => ({
      logs: counts.logs + Number(document.kind === "daily_log"),
      topics: counts.topics + Number(document.kind !== "daily_log"),
    }),
    { logs: 0, topics: 0 },
  );
}

function memoryDocumentMatches(
  document: MemoryDocument,
  filter: MemoryFilter | "index",
  query: string,
): boolean {
  const matchesFilter = filter === "all"
    || document.kind === filter
    || (document.kind === "topic" && document.type === filter);
  if (!matchesFilter) {
    return false;
  }
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return true;
  }
  return [document.title, document.description, document.path, document.type]
    .filter(Boolean)
    .join(" ")
    .toLowerCase()
    .includes(normalizedQuery);
}
