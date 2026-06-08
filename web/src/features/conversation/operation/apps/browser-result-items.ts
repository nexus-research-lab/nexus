import type { NexusOperationEvent } from "../operation-types";

export interface BrowserResultItem {
  title: string;
  url: string;
  snippet: string;
}

export function build_browser_result_items({
  event,
  lines,
  query,
}: {
  event: NexusOperationEvent;
  lines: string[];
  query: string;
}): BrowserResultItem[] {
  const structured_items = extract_structured_browser_result_items(event.result_preview, query);
  if (structured_items.length) {
    return structured_items.slice(0, 6);
  }

  const source_lines = lines.length
    ? lines
    : event.phase === "running"
      ? ["正在等待页面返回内容", "加载完成后会保留页面摘要和可回看记录。"]
      : [event.summary ?? query];

  return source_lines
    .map(clean_browser_result_line)
    .filter((line) => line.trim())
    .slice(0, 6)
    .map((line, index) => normalize_browser_result_line(line, query, index));
}

function extract_structured_browser_result_items(value: unknown, query: string): BrowserResultItem[] {
  if (value == null) {
    return [];
  }
  if (Array.isArray(value)) {
    return value.flatMap((item, index) => {
      const result = browser_result_item_from_record(item, query, index);
      if (result) {
        return [result];
      }
      return extract_structured_browser_result_items(item, query);
    });
  }
  if (typeof value !== "object") {
    return [];
  }

  const record = value as Record<string, unknown>;
  const direct_result = browser_result_item_from_record(record, query, 0);
  if (direct_result) {
    return [direct_result];
  }

  for (const key of ["results", "items", "entries", "organic_results", "content", "data"]) {
    const nested = extract_structured_browser_result_items(record[key], query);
    if (nested.length) {
      return nested;
    }
  }
  return [];
}

function browser_result_item_from_record(value: unknown, query: string, index: number): BrowserResultItem | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  const record = value as Record<string, unknown>;
  const url = read_browser_result_string(record, ["url", "link", "href", "uri"]);
  const title = read_browser_result_string(record, ["title", "name", "label"]);
  const snippet = read_browser_result_string(record, ["snippet", "summary", "description", "text", "content"]);
  if (!url && !title && !snippet) {
    return null;
  }
  return {
    title: title ?? (url ? readable_url_title(url) : index === 0 ? query : `结果 ${index + 1}`),
    url: url ?? `nexus-search://${encodeURIComponent(query)}/${index + 1}`,
    snippet: snippet ?? query,
  };
}

function read_browser_result_string(record: Record<string, unknown>, keys: string[]): string | null {
  for (const key of keys) {
    const value = record[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
    if (typeof value === "number" || typeof value === "boolean") {
      return String(value);
    }
  }
  return null;
}

function clean_browser_result_line(line: string): string {
  const trimmed = line.trim();
  if (trimmed === "[" || trimmed === "]" || trimmed === "{" || trimmed === "}") {
    return "";
  }
  return trimmed
    .replace(/,$/, "")
    .replace(/^["']/, "")
    .replace(/["']$/, "")
    .trim();
}

function normalize_browser_result_line(line: string, query: string, index: number): BrowserResultItem {
  const trimmed = line.trim();
  if (looks_like_url(trimmed)) {
    return {
      title: readable_url_title(trimmed),
      url: trimmed,
      snippet: query,
    };
  }

  const markdown_link = trimmed.match(/\[([^\]]+)]\((https?:\/\/[^)]+)\)/i);
  if (markdown_link) {
    return {
      title: markdown_link[1],
      url: markdown_link[2],
      snippet: trimmed.replace(markdown_link[0], "").replace(/^[-:\s]+/, "") || query,
    };
  }

  const url_match = trimmed.match(/https?:\/\/\S+/i);
  if (url_match) {
    const url = url_match[0].replace(/[),.;]+$/, "");
    return {
      title: trimmed.slice(0, url_match.index).replace(/^[-*\s]+|[-:\s]+$/g, "") || readable_url_title(url),
      url,
      snippet: trimmed.replace(url_match[0], "").replace(/^[-:\s]+/, "") || query,
    };
  }

  return {
    title: index === 0 ? query : `结果 ${index + 1}`,
    url: `nexus-search://${encodeURIComponent(query)}/${index + 1}`,
    snippet: trimmed,
  };
}

function readable_url_title(url: string): string {
  try {
    const parsed = new URL(url);
    const path = parsed.pathname.split("/").filter(Boolean).at(-1);
    return path ? `${parsed.hostname} / ${decodeURIComponent(path)}` : parsed.hostname;
  } catch {
    return url;
  }
}

function looks_like_url(value: string): boolean {
  return /^https?:\/\//i.test(value);
}
