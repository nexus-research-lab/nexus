import type { NexusOperationEvent } from "../operation-types";

export interface BrowserReaderParagraph {
  highlighted: boolean;
  text: string;
}

export function is_web_fetch_event(event: NexusOperationEvent): boolean {
  return event.tool_name === "WebFetch" || Boolean(read_browser_input_string(event, ["url", "uri", "link"]));
}

export function build_browser_reader_paragraphs({
  fallback,
  lines,
  markers,
  preview,
}: {
  fallback: string;
  lines: string[];
  markers: Array<string | null | undefined>;
  preview: unknown;
}): BrowserReaderParagraph[] {
  const source_lines = lines.length ? lines : extract_reader_lines(preview);
  const paragraphs = source_lines
    .map(sanitize_reader_line)
    .filter(is_readable_reader_line);
  const clean_markers = markers.map(normalize_match_text).filter(Boolean);
  const items = (paragraphs.length ? paragraphs.slice(0, 12) : [fallback || "页面内容已抓取，等待工具返回正文。"]);

  return items.map((text) => ({
    highlighted: lines.length > 0 || clean_markers.some((marker) => normalize_match_text(text).includes(marker)),
    text,
  }));
}

export function read_browser_input_string(event: NexusOperationEvent, keys: string[]): string | null {
  const input = event.input_preview;
  if (!input) {
    return null;
  }
  for (const key of keys) {
    const value = input[key];
    if (typeof value === "string" && value.trim()) {
      return value.trim();
    }
  }
  return null;
}

export function format_browser_origin(value: string): string {
  try {
    return new URL(value).hostname;
  } catch {
    return "工作区预览";
  }
}

export function looks_like_url(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function sanitize_reader_line(line: string): string {
  return line
    .trim()
    .replace(/^[\s"']+/, "")
    .replace(/[",']+\s*$/, "")
    .trim();
}

function is_readable_reader_line(line: string): boolean {
  if (!line || line === "{" || line === "}" || line === "[" || line === "]") {
    return false;
  }
  if (/^(content|error_code|is_error|stdout|stderr|exit_code|status|html)\s*:/i.test(line)) {
    return false;
  }
  if (/<\/?(html|body|script|style|head|meta|div|span)\b/i.test(line)) {
    return false;
  }
  if (/(0x[0-9a-f]{2},){6,}/i.test(line)) {
    return false;
  }
  return true;
}

function extract_reader_lines(value: unknown): string[] {
  if (value == null) {
    return [];
  }
  if (typeof value === "string") {
    return value.split(/\r?\n/);
  }
  if (Array.isArray(value)) {
    return value.flatMap(extract_reader_lines);
  }
  if (typeof value !== "object") {
    return [String(value)];
  }
  const record = value as Record<string, unknown>;
  for (const key of ["content", "text", "markdown", "summary", "result", "body"]) {
    const lines = extract_reader_lines(record[key]);
    if (lines.length) {
      return lines;
    }
  }
  return [];
}

function normalize_match_text(value: string | null | undefined): string {
  return (value ?? "").toLowerCase().replace(/\s+/g, " ").trim();
}
