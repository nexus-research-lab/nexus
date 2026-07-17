/**
 * INPUT: WebSearch, WebFetch, and browser-preview Operation Stage events.
 * OUTPUT: Stable agent-driven browser pages with live targets and truthful Reader snapshots.
 * POS: Pure event-to-browser projection; it never owns interactive tab navigation.
 */
import { getPreviewLines } from "../operation-preview";
import type { NexusOperationEvent } from "../operation-types";
import {
  buildBrowserReaderParagraphs,
  formatBrowserOrigin,
  isWebFetchEvent,
  looksLikeUrl,
  readBrowserInputString,
} from "./browser-reader-model";
import type { BrowserReaderParagraph } from "./browser-reader-model";
import { buildBrowserResultItems } from "./browser-result-items";
import type { BrowserResultItem } from "./browser-result-items";
import { buildBrowserSessionView } from "./browser-session";
import type { BrowserSessionView } from "./browser-session";

export interface BrowserReaderSnapshot {
  highlighted_count: number;
  origin: string;
  paragraphs: BrowserReaderParagraph[];
  prompt: string;
  url: string;
}

export interface BrowserPageSnapshot {
  address: string;
  event: NexusOperationEvent | null;
  id: string;
  iframe_url: string | null;
  kind: BrowserSessionView["page_kind"];
  presentation: "live" | "reader";
  query: string;
  reader: BrowserReaderSnapshot | null;
  reload_key: number;
  results: BrowserResultItem[];
  source: "agent" | "user";
  source_label: string;
  srcdoc: string | null;
  status: BrowserSessionView["status"];
  tab_title: string;
  target: string | null;
}

export function buildBrowserAgentPages({
  event,
  preview,
  query,
  raw_url_builder,
  related_events,
  target,
  web_url_builder,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  query: string;
  raw_url_builder?: (agent_id: string, path: string) => string;
  related_events: NexusOperationEvent[];
  target?: string | null;
  web_url_builder?: (url: string) => string;
}): BrowserPageSnapshot[] {
  const events = ordered_unique_events([...related_events, event]);
  return events.map((item) => {
    const is_current = item.id === event.id;
    return build_agent_page({
      event: item,
      preview: is_current ? (preview ?? item.result_preview ?? item.summary) : (item.result_preview ?? item.summary),
      query: is_current ? query : browser_query_for_event(item),
      raw_url_builder,
      target: is_current ? (target ?? item.target) : item.target,
      web_url_builder,
    });
  });
}

export function createBrowserStartPage(id: string, source: "agent" | "user" = "user"): BrowserPageSnapshot {
  return {
    address: "about:blank",
    event: null,
    id,
    iframe_url: null,
    kind: "start",
    presentation: "live",
    query: "",
    reader: null,
    reload_key: 0,
    results: [],
    source,
    source_label: "起始页",
    srcdoc: null,
    status: { label: "就绪", tone: "idle" },
    tab_title: "起始页",
    target: null,
  };
}

export function createBrowserUserPage({
  id,
  input,
  reader,
  web_url_builder,
}: {
  id: string;
  input: string;
  reader?: BrowserReaderSnapshot | null;
  web_url_builder?: (url: string) => string;
}): BrowserPageSnapshot {
  const normalized = normalizeBrowserAddress(input);
  if (!normalized) {
    return createBrowserStartPage(id);
  }
  if (!looksLikeUrl(normalized)) {
    return {
      address: `navi://search?${new URLSearchParams({ q: normalized }).toString()}`,
      event: null,
      id,
      iframe_url: null,
      kind: "search",
      presentation: "live",
      query: normalized,
      reader: null,
      reload_key: 0,
      results: [],
      source: "user",
      source_label: "Nexus Search",
      srcdoc: null,
      status: { label: "本轮无搜索记录", tone: "idle" },
      tab_title: compact_title(normalized),
      target: normalized,
    };
  }
  return {
    address: normalized,
    event: null,
    id,
    iframe_url: web_url_builder ? web_url_builder(normalized) : normalized,
    kind: "web",
    presentation: "live",
    query: normalized,
    reader: reader ?? null,
    reload_key: 0,
    results: [],
    source: "user",
    source_label: "网页",
    srcdoc: null,
    status: { label: "正在载入", tone: "loading" },
    tab_title: readable_url_title(normalized),
    target: normalized,
  };
}

export function normalizeBrowserAddress(value: string): string | null {
  const normalized = value.trim();
  if (!normalized || normalized === "about:blank") {
    return null;
  }
  if (looksLikeUrl(normalized)) {
    return normalized;
  }
  if (/^(localhost|127\.0\.0\.1|\[::1\])(?::\d+)?(?:\/|$)/i.test(normalized)) {
    return `http://${normalized}`;
  }
  if (/^[a-z0-9](?:[a-z0-9-]*\.)+[a-z]{2,}(?::\d+)?(?:\/\S*)?$/i.test(normalized)) {
    return `https://${normalized}`;
  }
  return normalized;
}

function build_agent_page({
  event,
  preview,
  query,
  raw_url_builder,
  target,
  web_url_builder,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  query: string;
  raw_url_builder?: (agent_id: string, path: string) => string;
  target?: string | null;
  web_url_builder?: (url: string) => string;
}): BrowserPageSnapshot {
  const session = buildBrowserSessionView({
    event,
    force_search: is_web_search_event(event),
    preview,
    query,
    raw_url_builder,
    target,
    web_url_builder,
  });
  const lines = getPreviewLines(preview ?? event.result_preview ?? event.summary, 80);
  const reader = build_reader_snapshot(event, preview, query);
  return {
    address: session.display_url || "about:blank",
    event,
    id: `agent:${event.tool_use_id ?? event.id}`,
    iframe_url: session.iframe_url,
    kind: session.page_kind,
    presentation: "live",
    query,
    reader,
    reload_key: 0,
    results: session.page_kind === "search"
      ? buildBrowserResultItems({ event, lines, query })
      : [],
    source: "agent",
    source_label: session.source_label,
    srcdoc: session.srcdoc,
    status: session.status,
    tab_title: session.tab_title,
    target: target ?? event.target ?? null,
  };
}

function is_web_search_event(event: NexusOperationEvent): boolean {
  return event.tool_name?.replace(/[^a-z]/gi, "").toLowerCase() === "websearch";
}

function build_reader_snapshot(
  event: NexusOperationEvent,
  preview: unknown,
  query: string,
): BrowserReaderSnapshot | null {
  if (!isWebFetchEvent(event)) {
    return null;
  }
  const url = readBrowserInputString(event, ["url", "uri", "link"])
    ?? (looksLikeUrl(query) ? query : null);
  if (!url) {
    return null;
  }
  const prompt = readBrowserInputString(event, ["prompt", "question", "query"])
    ?? event.summary
    ?? "";
  const paragraphs = buildBrowserReaderParagraphs({
    fallback: event.summary ?? prompt,
    lines: [],
    markers: [event.summary, prompt],
    preview,
  });
  return {
    highlighted_count: paragraphs.filter((paragraph) => paragraph.highlighted).length,
    origin: formatBrowserOrigin(url),
    paragraphs,
    prompt,
    url,
  };
}

function browser_query_for_event(event: NexusOperationEvent): string {
  return readBrowserInputString(event, ["url", "query", "q", "search_query", "prompt"])
    ?? event.target
    ?? "about:blank";
}

function ordered_unique_events(events: NexusOperationEvent[]): NexusOperationEvent[] {
  const by_id = new Map<string, NexusOperationEvent>();
  [...events]
    .sort((left, right) => left.updated_at - right.updated_at)
    .forEach((event) => by_id.set(event.tool_use_id ? `tool:${event.tool_use_id}` : `event:${event.id}`, event));
  return [...by_id.values()];
}

function readable_url_title(value: string): string {
  try {
    return new URL(value).hostname || value;
  } catch {
    return value;
  }
}

function compact_title(value: string): string {
  const normalized = value.trim() || "起始页";
  return normalized.length > 24 ? `${normalized.slice(0, 23)}…` : normalized;
}
