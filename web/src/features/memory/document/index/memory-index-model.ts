export interface MemoryIndexEntry {
  description: string;
  path: string;
  title: string;
}

const INDEX_ENTRY_PATTERN = /^\s*-\s+\[([^\]]+)]\(([^)]+\.md)(?:#[^)]*)?\)\s*(?:[—–-]\s*)?(.*)$/gm;

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

function normalizeMemoryPath(path: string): string {
  return path.trim().replace(/^<|>$/g, "").replace(/^\.\//, "").replaceAll("\\", "/");
}
