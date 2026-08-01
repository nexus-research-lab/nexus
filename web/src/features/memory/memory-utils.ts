const FRONTMATTER_PATTERN = /^---\r?\n[\s\S]*?\r?\n---\r?\n?/;

export const MEMORY_STALE_AFTER_DAYS = 1;

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
