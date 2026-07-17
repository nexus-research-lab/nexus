/**
 * INPUT: A browser-capable Operation Stage event and its resolved preview target.
 * OUTPUT: A truthful browser page descriptor without claiming cross-origin render success.
 * POS: Shared projection boundary between tool events and the Navi browser app.
 */
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

export function buildBrowserSessionView({
  event,
  force_search = false,
  preview,
  query,
  raw_url_builder,
  target,
  web_url_builder,
}: {
  event: NexusOperationEvent;
  force_search?: boolean;
  preview: unknown;
  query: string;
  raw_url_builder?: (agent_id: string, path: string) => string;
  target?: string | null;
  web_url_builder?: (url: string) => string;
}): BrowserSessionView {
  const raw_url = build_workspace_raw_url(event.agent_id, target ?? event.target, raw_url_builder);
  const html_content = !force_search && typeof preview === "string" && looks_like_html(preview)
    ? preview
    : null;
  const srcdoc = html_content && raw_url
    ? inject_workspace_base(html_content, raw_url)
    : html_content;
  const url = !force_search && looksLikeUrl(query) ? query : null;
  const iframe_url = raw_url ?? (srcdoc ? null : build_web_iframe_url(url, web_url_builder));
  const has_live_view = Boolean(srcdoc || iframe_url);
  const display_url = browser_display_url({ query, raw_url, srcdoc, target, url });
  const page_kind = force_search
    ? "search"
    : browser_page_kind({ display_url, raw_url, srcdoc });

  return {
    display_url,
    iframe_url,
    page_kind,
    source_label: browser_source_label(page_kind),
    srcdoc,
    status: browser_status_for_event(event, has_live_view, page_kind),
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
  page_kind: BrowserSessionView["page_kind"],
): BrowserSessionView["status"] {
  if (event.phase === "running") {
    return {
      label: page_kind === "web" ? "正在访问" : (has_live_view ? "页面运行中" : "正在加载"),
      tone: "loading",
    };
  }
  if (event.phase === "error") {
    return { label: "加载失败", tone: "error" };
  }
  if (event.phase === "done") {
    return {
      label: page_kind === "web" ? "内容已获取" : (has_live_view ? "页面已就绪" : "已生成摘要"),
      tone: "ready",
    };
  }
  return { label: PHASE_LABEL[event.phase], tone: "idle" };
}

function browser_display_url({
  query,
  raw_url,
  srcdoc,
  target,
  url,
}: {
  query: string;
  raw_url: string | null;
  srcdoc: string | null;
  target?: string | null;
  url: string | null;
}): string {
  if (raw_url) {
    return raw_url;
  }
  if (srcdoc) {
    return url ?? target ?? query;
  }
  return url ?? query;
}

function browser_page_kind({
  display_url,
  raw_url,
  srcdoc,
}: {
  display_url: string;
  raw_url: string | null;
  srcdoc: string | null;
}): BrowserSessionView["page_kind"] {
  if (raw_url) {
    return "workspace";
  }
  if (srcdoc) {
    return "embedded";
  }
  if (display_url === "about:blank") {
    return "start";
  }
  if (looksLikeUrl(display_url)) {
    return "web";
  }
  return "search";
}

function build_web_iframe_url(
  url: string | null,
  web_url_builder?: (url: string) => string,
): string | null {
  if (!url) {
    return null;
  }
  return web_url_builder ? web_url_builder(url) : url;
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
  return "Nexus Search";
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
  if (!looksLikeUrl(value)) {
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
  if (!trimmed || looksLikeUrl(trimmed)) {
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

function looksLikeUrl(value: string): boolean {
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
  if (raw_url_builder) {
    return raw_url_builder(agent_id, path);
  }
  const encoded_path = path
    .split("/")
    .filter(Boolean)
    .map((segment) => encodeURIComponent(segment))
    .join("/");
  return `/nexus/v1/agents/${encodeURIComponent(agent_id)}/workspace/site/${encoded_path}`;
}

function inject_workspace_base(content: string, raw_url: string): string {
  if (/<base\b/i.test(content)) {
    return content;
  }
  const path_end = raw_url.lastIndexOf("/");
  const base_url = path_end >= 0 ? raw_url.slice(0, path_end + 1) : raw_url;
  const base_tag = `<base href="${escape_html_attribute(base_url)}">`;
  if (/<head(\s[^>]*)?>/i.test(content)) {
    return content.replace(/<head(\s[^>]*)?>/i, (match) => `${match}${base_tag}`);
  }
  if (/<html(\s[^>]*)?>/i.test(content)) {
    return content.replace(/<html(\s[^>]*)?>/i, (match) => `${match}<head>${base_tag}</head>`);
  }
  return `${base_tag}${content}`;
}

function escape_html_attribute(value: string): string {
  return value
    .replace(/&/g, "&amp;")
    .replace(/"/g, "&quot;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;");
}

function normalize_workspace_relative_path(target?: string | null): string | null {
  const path = target?.trim();
  if (!path || looksLikeUrl(path) || path.startsWith("/") || path.includes("..")) {
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
