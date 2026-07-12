import type { MemoryDocument } from "@/types/memory/memory";

const FRONTMATTER_PATTERN = /^---\r?\n[\s\S]*?\r?\n---\r?\n?/;

export const MEMORY_STALE_AFTER_DAYS = 1;

export function isIndexedMemoryTopic(document: MemoryDocument): boolean {
  return document.indexed && document.kind === "topic";
}

export function stripMemoryFrontmatter(content: string): string {
  return content.replace(FRONTMATTER_PATTERN, "").trim();
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
