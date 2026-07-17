/**
 * INPUT: WebFetch previews, tool intent markers, and fallback text.
 * OUTPUT: Clean Reader paragraphs that preserve fetched article meaning and intent matches.
 * POS: Pure WebFetch-to-Reader normalization shared by Navi page projection.
 */
import type { NexusOperationEvent } from "../operation-types";

export interface BrowserReaderParagraph {
  highlighted: boolean;
  text: string;
}

export function isWebFetchEvent(event: NexusOperationEvent): boolean {
  return event.tool_name === "WebFetch" || Boolean(readBrowserInputString(event, ["url", "uri", "link"]));
}

export function buildBrowserReaderParagraphs({
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
  const clean_markers = markers.map(normalize_match_text).filter(Boolean);
  const marker_tokens = extract_marker_tokens(clean_markers);
  const article_lines = slice_reader_article(source_lines);
  const paragraphs = article_lines
    .map(sanitize_reader_line)
    .filter((line) => is_readable_reader_line(line, clean_markers));
  const items = (paragraphs.length ? paragraphs.slice(0, 12) : [fallback || "页面内容已抓取，等待工具返回正文。"]);

  return items.map((text) => ({
    highlighted: matches_reader_marker(text, clean_markers, marker_tokens),
    text,
  }));
}

export function readBrowserInputString(event: NexusOperationEvent, keys: string[]): string | null {
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

export function formatBrowserOrigin(value: string): string {
  try {
    return new URL(value).hostname;
  } catch {
    return "工作区预览";
  }
}

export function looksLikeUrl(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function sanitize_reader_line(line: string): string {
  return line
    .trim()
    .replace(/^[\s"']+/, "")
    .replace(/[",']+\s*$/, "")
    .replace(/^#{1,6}\s+/, "")
    .replace(/!\[[^\]]*]\([^)]*\)/g, "")
    .replace(/\[([^\]]+)]\((?:https?:\/\/|\/|#)[^)]*\)/g, "$1")
    .replace(/\\(?=\s|$)/g, "")
    .replace(/^[-*]\s+/, "• ")
    .trim();
}

function is_readable_reader_line(line: string, markers: string[]): boolean {
  if (!line || line === "{" || line === "}" || line === "[" || line === "]") {
    return false;
  }
  const prompt_match = line.match(/^prompt:\s*(.*)$/i);
  if (prompt_match && markers.some((marker) => marker === normalize_match_text(prompt_match[1]))) {
    return false;
  }
  if (/^(content|error_code|is_error|stdout|stderr|exit_code|status|html)"?\s*:/i.test(line)) {
    return false;
  }
  if (/<\/?(html|body|script|style|head|meta|div|span)\b/i.test(line)) {
    return false;
  }
  if (/(0x[0-9a-f]{2},){6,}/i.test(line)) {
    return false;
  }
  if (line === "/" || /^!\[[^\]]*]\([^)]*\)$/.test(line)) {
    return false;
  }
  if (/^\s*-?\s*\[[^\]]+]\((?:#|\/)[^)]*\)\s*$/.test(line)) {
    return false;
  }
  if (/skip to (?:main content|footer)/i.test(line)) {
    return false;
  }
  return true;
}

function slice_reader_article(lines: string[]): string[] {
  const first_heading = lines.findIndex((line) => /^\s*#\s+\S/.test(line));
  return first_heading > 0 ? lines.slice(first_heading) : lines;
}

function extract_reader_lines(value: unknown): string[] {
  if (value == null) {
    return [];
  }
  if (typeof value === "string") {
    const parsed = parse_reader_envelope(value);
    if (parsed !== null) {
      return extract_reader_lines(parsed);
    }
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

function parse_reader_envelope(value: string): unknown | null {
  const trimmed = value.trim();
  if (!trimmed || !["{", "[", '"'].includes(trimmed[0])) {
    return null;
  }
  try {
    const parsed = JSON.parse(trimmed) as unknown;
    return typeof parsed === "string" && parsed === value ? null : parsed;
  } catch {
    return null;
  }
}

function normalize_match_text(value: string | null | undefined): string {
  return (value ?? "").toLowerCase().replace(/\s+/g, " ").trim();
}

function extract_marker_tokens(markers: string[]): string[] {
  const stop_words = new Set([
    "about", "after", "agent", "anything", "especially", "find", "focus", "from",
    "into", "page", "relevant", "that", "their", "this", "with",
  ]);
  return [...new Set(markers.flatMap((marker) => (
    marker.match(/[a-z0-9][a-z0-9-]{3,}|[\p{Script=Han}]{2,}/giu) ?? []
  )).map((token) => token.toLowerCase()).filter((token) => !stop_words.has(token)))];
}

function matches_reader_marker(
  text: string,
  markers: string[],
  marker_tokens: string[],
): boolean {
  const normalized = normalize_match_text(text);
  if (markers.some((marker) => marker.length <= 96 && normalized.includes(marker))) {
    return true;
  }
  let matches = 0;
  for (const token of marker_tokens) {
    if (normalized.includes(token)) {
      matches += 1;
    }
  }
  return matches >= Math.min(2, marker_tokens.length || 2);
}
