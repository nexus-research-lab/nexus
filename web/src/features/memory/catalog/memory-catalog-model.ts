import type { TranslationKey } from "@/shared/i18n/messages";
import { matchesUiSearchFields } from "@/shared/ui/form/search-query";
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
  emptyFilterVisible: boolean;
  emptyMemoryVisible: boolean;
  sections: MemoryCatalogSection[];
  selectedDocument: MemoryDocument | null;
  truncated: boolean;
}

export interface MemoryCatalogRow {
  document: MemoryDocument;
  isSelected: boolean;
}

export interface MemoryCatalogSection {
  countVisible: boolean;
  key: "index" | "documents";
  labelKey: TranslationKey;
  rows: MemoryCatalogRow[];
}

type MemoryFilterMatcher = (document: MemoryDocument) => boolean;

const FILTER_MATCHER_BY_KEY: Readonly<Record<MemoryFilter, MemoryFilterMatcher>> = {
  all: () => true,
  daily_log: (document) => document.kind === "daily_log",
  feedback: (document) => document.kind === "topic" && document.type === "feedback",
  project: (document) => document.kind === "topic" && document.type === "project",
  reference: (document) => document.kind === "topic" && document.type === "reference",
  user: (document) => document.kind === "topic" && document.type === "user",
};

export function projectMemoryCatalog(
  snapshot: MemorySnapshot | null,
  selectedPath: string,
  filter: MemoryFilter,
  query: string,
): MemoryCatalogProjection {
  const allDocuments = getAllMemoryDocuments(snapshot);
  const sections = buildMemoryCatalogSections(
    snapshot,
    selectedPath,
    filter,
    query,
  );
  return {
    allDocuments,
    emptyFilterVisible: sections.length === 0,
    emptyMemoryVisible: isEmptyMemorySnapshot(snapshot),
    sections,
    selectedDocument: findSelectedMemoryDocument(allDocuments, selectedPath),
    truncated: isTruncatedMemorySnapshot(snapshot),
  };
}

export function resolveSelectedMemoryPath(
  snapshot: MemorySnapshot,
  currentPath: string,
): string {
  const documents = getAllMemoryDocuments(snapshot);
  return documents.some((document) => document.path === currentPath)
    ? currentPath
    : snapshot.documents[0]?.path ?? snapshot.index?.path ?? "";
}

function getAllMemoryDocuments(snapshot: MemorySnapshot | null): MemoryDocument[] {
  return snapshot
    ? [snapshot.index, ...snapshot.documents].filter(isMemoryDocument)
    : [];
}

function isMemoryDocument(
  document: MemoryDocument | null | undefined,
): document is MemoryDocument {
  return Boolean(document);
}

function isEmptyMemorySnapshot(snapshot: MemorySnapshot | null): boolean {
  return snapshot?.layout === "empty";
}

function isTruncatedMemorySnapshot(snapshot: MemorySnapshot | null): boolean {
  return Boolean(snapshot?.truncated);
}

function findSelectedMemoryDocument(
  documents: MemoryDocument[],
  selectedPath: string,
): MemoryDocument | null {
  return documents.find((document) => document.path === selectedPath) ?? null;
}

function memoryDocumentMatches(
  document: MemoryDocument,
  filter: MemoryFilter,
  query: string,
): boolean {
  return FILTER_MATCHER_BY_KEY[filter](document)
    && memoryDocumentMatchesQuery(document, query);
}

function memoryDocumentMatchesQuery(
  document: MemoryDocument,
  query: string,
): boolean {
  return matchesUiSearchFields(query, [
    document.title,
    document.description,
    document.path,
    document.type,
  ]);
}

function buildMemoryCatalogSections(
  snapshot: MemorySnapshot | null,
  selectedPath: string,
  filter: MemoryFilter,
  query: string,
): MemoryCatalogSection[] {
  const indexRows = snapshot?.index && memoryDocumentMatchesQuery(snapshot.index, query)
    ? [projectMemoryCatalogRow(snapshot.index, selectedPath)]
    : [];
  const documentRows = (snapshot?.documents ?? [])
    .filter((document) => memoryDocumentMatches(document, filter, query))
    .map((document) => projectMemoryCatalogRow(document, selectedPath));
  const sections: MemoryCatalogSection[] = [
    {
      countVisible: false,
      key: "index",
      labelKey: "capability.memory_index",
      rows: indexRows,
    },
    {
      countVisible: true,
      key: "documents",
      labelKey: "capability.memory_documents",
      rows: documentRows,
    },
  ];
  return sections.filter((section) => section.rows.length > 0);
}

function projectMemoryCatalogRow(
  document: MemoryDocument,
  selectedPath: string,
): MemoryCatalogRow {
  return {
    document,
    isSelected: document.path === selectedPath,
  };
}
