import type { MemoryDocument } from "@/types/memory/memory";

export type MemoryFilter = "all" | "index" | "user" | "feedback" | "project" | "reference" | "daily_log";

export interface MemoryIndexEntry {
  description: string;
  path: string;
  title: string;
}

const FRONTMATTER_PATTERN = /^---\r?\n[\s\S]*?\r?\n---\r?\n?/;
const INDEX_ENTRY_PATTERN = /^\s*-\s+\[([^\]]+)]\(([^)]+\.md)(?:#[^)]*)?\)\s*(?:[—–-]\s*)?(.*)$/gm;

export function stripMemoryFrontmatter(content: string): string {
  return content.replace(FRONTMATTER_PATTERN, "").trim();
}

export function parseMemoryIndexEntries(content: string): MemoryIndexEntry[] {
  const entries: MemoryIndexEntry[] = [];
  for (const match of content.matchAll(INDEX_ENTRY_PATTERN)) {
    const path = normalizeMemoryPath(match[2]);
    if (!path.startsWith("memory/")) {
      continue;
    }
    entries.push({
      description: match[3]?.trim() ?? "",
      path,
      title: match[1].trim(),
    });
  }
  return entries;
}

export function memoryDocumentMatches(
  document: MemoryDocument,
  filter: MemoryFilter,
  query: string,
): boolean {
  const matchesFilter = filter === "all"
    || (filter === "index" && document.kind === "index")
    || (filter === "daily_log" && document.kind === "daily_log")
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

export function memoryAgeDays(modifiedAt: string, now = Date.now()): number {
  const timestamp = Date.parse(modifiedAt);
  if (!Number.isFinite(timestamp)) {
    return 0;
  }
  return Math.max(0, Math.floor((now - timestamp) / 86_400_000));
}

export function formatMemoryModifiedTime(modifiedAt: string, locale: string): string {
  const timestamp = Date.parse(modifiedAt);
  if (!Number.isFinite(timestamp)) {
    return "-";
  }
  return new Intl.DateTimeFormat(locale === "zh" ? "zh-CN" : "en-US", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(timestamp);
}

export function formatMemoryFileSize(size: number): string {
  if (size < 1024) {
    return `${size} B`;
  }
  if (size < 1024 * 1024) {
    return `${(size / 1024).toFixed(size < 10 * 1024 ? 1 : 0)} KB`;
  }
  return `${(size / (1024 * 1024)).toFixed(1)} MB`;
}

function normalizeMemoryPath(path: string): string {
  return path.trim().replace(/^<|>$/g, "").replace(/^\.\//, "").replaceAll("\\", "/");
}
