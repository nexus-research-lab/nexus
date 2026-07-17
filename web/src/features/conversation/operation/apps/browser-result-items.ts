import type { NexusOperationEvent } from "../operation-types";

export interface BrowserResultItem {
  title: string;
  url: string | null;
  snippet: string;
  kind: "link" | "summary";
}

export function buildBrowserResultItems({
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

  const clean_lines = source_lines
    .map(clean_browser_result_line)
    .filter((line) => line.trim());
  return group_browser_result_lines(clean_lines, query).slice(0, 6);
}

function group_browser_result_lines(lines: string[], query: string): BrowserResultItem[] {
  const items: BrowserResultItem[] = [];
  const leading_summary: string[] = [];

  lines.forEach((line, index) => {
    const item = normalize_browser_result_line(line, query, index);
    if (item.kind === "link") {
      if (leading_summary.length) {
        item.snippet = leading_summary.join(" ");
        leading_summary.length = 0;
      }
      items.push(item);
      return;
    }

    const previous = items.at(-1);
    if (previous?.kind === "link") {
      previous.snippet = previous.snippet === query
        ? item.snippet
        : `${previous.snippet} ${item.snippet}`;
      return;
    }
    leading_summary.push(item.snippet);
  });

  if (leading_summary.length) {
    items.push({
      title: query,
      url: null,
      snippet: leading_summary.join(" "),
      kind: "summary",
    });
  }
  return items;
}

function extract_structured_browser_result_items(value: unknown, query: string): BrowserResultItem[] {
  if (value == null) {
    return [];
  }
  if (typeof value === "string") {
    const parsed = parse_json_result_payload(value);
    return parsed == null
      ? []
      : extract_structured_browser_result_items(parsed, query);
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
  for (const key of ["results", "items", "entries", "organic_results", "content", "data"]) {
    const nested = extract_structured_browser_result_items(record[key], query);
    if (nested.length) {
      return nested;
    }
  }
  const direct_result = browser_result_item_from_record(record, query, 0);
  return direct_result ? [direct_result] : [];
}

function parse_json_result_payload(value: string): unknown | null {
  const trimmed = value.trim();
  const candidate = trimmed.match(/^```(?:json)?\s*([\s\S]*?)\s*```$/i)?.[1] ?? trimmed;
  if (!candidate.startsWith("{") && !candidate.startsWith("[")) {
    return null;
  }
  try {
    return JSON.parse(candidate);
  } catch {
    return null;
  }
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
    url,
    snippet: snippet ?? query,
    kind: url ? "link" : "summary",
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
  if (/^(links|web search results for query):?/i.test(trimmed)) {
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
  if (looksLikeUrl(trimmed)) {
    return {
      title: readable_url_title(trimmed),
      url: trimmed,
      snippet: query,
      kind: "link",
    };
  }

  const markdown_link = trimmed.match(/\[([^\]]+)]\((https?:\/\/[^)]+)\)/i);
  if (markdown_link) {
    return {
      title: markdown_link[1],
      url: markdown_link[2],
      snippet: trimmed.replace(markdown_link[0], "").replace(/^[-:\s]+/, "") || query,
      kind: "link",
    };
  }

  const url_match = trimmed.match(/https?:\/\/\S+/i);
  if (url_match) {
    const url = url_match[0].replace(/[),.;]+$/, "");
    return {
      title: trimmed.slice(0, url_match.index).replace(/^[-*\s]+|[-:\s]+$/g, "") || readable_url_title(url),
      url,
      snippet: trimmed.replace(url_match[0], "").replace(/^[-:\s]+/, "") || query,
      kind: "link",
    };
  }

  return {
    title: index === 0 ? "工具返回摘要" : `摘要片段 ${index + 1}`,
    url: null,
    snippet: trimmed,
    kind: "summary",
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

function looksLikeUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}
