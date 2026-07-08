import type { NexusOperationEvent, OperationPhase } from "../operation-types";

const PHASE_LABEL: Record<OperationPhase, string> = {
  queued: "排队中",
  running: "执行中",
  waiting: "等待确认",
  done: "已完成",
  error: "失败",
  cancelled: "已中断",
};

export interface BrowserSessionView {
  display_url: string;
  iframe_url: string | null;
  page_kind: "embedded" | "workspace" | "web" | "search" | "start";
  source_label: string;
  srcdoc: string | null;
  status: { label: string; tone: "loading" | "ready" | "error" | "idle" };
  tab_title: string;
  url: string | null;
}

export function build_browser_session_view({
  event,
  preview,
  query,
  raw_url_builder,
  target,
}: {
  event: NexusOperationEvent;
  preview: unknown;
  query: string;
  raw_url_builder?: (agent_id: string, path: string) => string;
  target?: string | null;
}): BrowserSessionView {
  const srcdoc = typeof preview === "string" && looks_like_html(preview) ? preview : null;
  const raw_url = build_workspace_raw_url(event.agent_id, target ?? event.target, raw_url_builder);
  const url = looks_like_url(query) ? query : null;
  const iframe_url = srcdoc ? null : raw_url;
  const has_live_view = Boolean(srcdoc || iframe_url);
  const display_url = browser_display_url({ iframe_url, query, srcdoc, target });
  const page_kind = browser_page_kind({ display_url, iframe_url, srcdoc });

  return {
    display_url,
    iframe_url,
    page_kind,
    source_label: browser_source_label(page_kind),
    srcdoc,
    status: browser_status_for_event(event, has_live_view),
    tab_title: browser_tab_title({
      display_url,
      page_kind,
      query,
      srcdoc,
      target,
    }),
    url,
  };
}

function browser_status_for_event(
  event: NexusOperationEvent,
  has_live_view: boolean,
): BrowserSessionView["status"] {
  if (event.phase === "running") {
    return { label: has_live_view ? "页面运行中" : "正在加载", tone: "loading" };
  }
  if (event.phase === "error") {
    return { label: "加载失败", tone: "error" };
  }
  if (event.phase === "done") {
    return { label: has_live_view ? "页面已就绪" : "已生成摘要", tone: "ready" };
  }
  return { label: PHASE_LABEL[event.phase], tone: "idle" };
}

function browser_display_url({
  iframe_url,
  query,
  srcdoc,
  target,
}: {
  iframe_url: string | null;
  query: string;
  srcdoc: string | null;
  target?: string | null;
}): string {
  if (iframe_url) {
    return iframe_url;
  }
  if (srcdoc) {
    return target ?? query;
  }
  return query;
}

function browser_page_kind({
  display_url,
  iframe_url,
  srcdoc,
}: {
  display_url: string;
  iframe_url: string | null;
  srcdoc: string | null;
}): BrowserSessionView["page_kind"] {
  if (srcdoc) {
    return "embedded";
  }
  if (iframe_url?.startsWith("/nexus/")) {
    return "workspace";
  }
  if (display_url === "about:blank") {
    return "start";
  }
  if (looks_like_url(display_url)) {
    return "web";
  }
  return "search";
}

function browser_source_label(page_kind: BrowserSessionView["page_kind"]): string {
  if (page_kind === "embedded") {
    return "内嵌页面";
  }
  if (page_kind === "workspace") {
    return "工作区";
  }
  if (page_kind === "web") {
    return "网页";
  }
  if (page_kind === "start") {
    return "起始页";
  }
  return "摘要";
}

function browser_tab_title({
  display_url,
  page_kind,
  query,
  srcdoc,
  target,
}: {
  display_url: string;
  page_kind: BrowserSessionView["page_kind"];
  query: string;
  srcdoc: string | null;
  target?: string | null;
}): string {
  const html_title = srcdoc ? extract_html_title(srcdoc) : null;
  const fallback = html_title
    ?? basename(target)
    ?? readable_url_title(display_url)
    ?? query
    ?? "起始页";
  if (page_kind === "search") {
    return `搜索：${compact_title(fallback)}`;
  }
  if (page_kind === "start") {
    return "起始页";
  }
  return compact_title(fallback);
}

function extract_html_title(value: string): string | null {
  const match = value.match(/<title[^>]*>(.*?)<\/title>/is);
  const title = match?.[1]?.replace(/\s+/g, " ").trim();
  return title || null;
}

function readable_url_title(value: string): string | null {
  if (!looks_like_url(value)) {
    return null;
  }
  try {
    const parsed = new URL(value);
    return parsed.hostname || value;
  } catch {
    return value;
  }
}

function basename(value?: string | null): string | null {
  const trimmed = value?.trim();
  if (!trimmed || looks_like_url(trimmed)) {
    return null;
  }
  return trimmed.split(/[\\/]/).filter(Boolean).at(-1) ?? trimmed;
}

function compact_title(value: string): string {
  const normalized = value.trim() || "起始页";
  return normalized.length > 24 ? `${normalized.slice(0, 23)}…` : normalized;
}

function looks_like_html(value: string): boolean {
  return /<!doctype html|<html[\s>]|<body[\s>]|<script[\s>]/i.test(value);
}

function looks_like_url(value: string): boolean {
  return /^https?:\/\//i.test(value);
}

function build_workspace_raw_url(
  agent_id: string,
  target?: string | null,
  raw_url_builder?: (agent_id: string, path: string) => string,
): string | null {
  const path = normalize_workspace_relative_path(target);
  if (!path || !/\.(html?|xhtml)$/i.test(path)) {
    return null;
  }
  return raw_url_builder?.(agent_id, path) ?? `/nexus/v1/agents/${agent_id}/workspace/file/raw?${new URLSearchParams({ path }).toString()}`;
}

function normalize_workspace_relative_path(target?: string | null): string | null {
  const path = target?.trim();
  if (!path || looks_like_url(path) || path.startsWith("/") || path.includes("..")) {
    return null;
  }
  const normalized = path.replace(/^\.\/+/, "");
  if (
    !normalized ||
    normalized.startsWith(".agents/") ||
    normalized.startsWith(".claude/") ||
    normalized.startsWith(".git/")
  ) {
    return null;
  }
  return normalized;
}
