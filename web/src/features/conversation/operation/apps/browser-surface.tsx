import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  BookOpen,
  ExternalLink,
  FileText,
  Globe2,
  Loader2,
  PanelLeft,
  Plus,
  RefreshCw,
  Search,
  Share2,
  ShieldCheck,
} from "lucide-react";
import type { ReactNode } from "react";

import { HtmlFilePreview } from "@/features/conversation/shared/editor/media/html-file-preview";
import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent/agent-api";
import { cn } from "@/shared/ui/class-name";

import type {
  NexusOperationEvent,
  OperationPhase,
} from "../operation-types";
import {
  buildBrowserReaderParagraphs,
  formatBrowserOrigin,
  isWebFetchEvent,
  looksLikeUrl,
  readBrowserInputString,
} from "./browser-reader-model";
import { buildBrowserResultItems } from "./browser-result-items";
import { buildBrowserSessionView } from "./browser-session";

const PHASE_LABEL: Record<OperationPhase, string> = {
  queued: "排队中",
  running: "执行中",
  waiting: "等待确认",
  done: "已完成",
  error: "失败",
  cancelled: "已中断",
};

export function BrowserSurface({
  event,
  lines,
  preview,
  query,
  target,
}: {
  event: NexusOperationEvent;
  lines: string[];
  preview: unknown;
  query: string;
  target?: string | null;
}) {
  const session = buildBrowserSessionView({
    event,
    preview,
    query,
    raw_url_builder: getWorkspaceFilePreviewUrl,
    target,
  });

  return (
    <div className="flex min-h-0 min-w-0 max-w-full flex-1 flex-col overflow-hidden bg-[#f7f9fc] shadow-[inset_0_1px_0_rgba(255,255,255,0.82)]">
      <BrowserChromeHeader
        displayUrl={session.display_url}
        event={event}
        sourceLabel={session.source_label}
        status={session.status}
        tabTitle={session.tab_title}
      />

      <BrowserViewport
        event={event}
        iframeUrl={session.iframe_url}
        lines={lines}
        pageKind={session.page_kind}
        preview={preview}
        query={query}
        srcdoc={session.srcdoc}
        target={target}
      />
    </div>
  );
}

function BrowserChromeHeader({
  displayUrl,
  event,
  sourceLabel,
  status,
  tabTitle,
}: {
  displayUrl: string;
  event: NexusOperationEvent;
  sourceLabel: string;
  status: { label: string; tone: "loading" | "ready" | "error" | "idle" };
  tabTitle: string;
}) {
  return (
    <div className="border-b border-(--divider-subtle-color) bg-[rgba(248,250,253,0.88)]">
      <div className="flex min-w-0 items-end gap-1.5 px-3 pt-2">
        <div className="flex min-w-0 max-w-[52%] items-center gap-1.5 rounded-t-[10px] border border-b-0 border-(--divider-subtle-color) bg-white/72 px-3 py-1.5 text-[10px] font-bold text-(--text-strong)">
          <Globe2 className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <span className="truncate">{tabTitle}</span>
        </div>
      </div>
      <div className="flex min-w-0 items-center gap-2 px-3 py-2">
        <div className="flex shrink-0 items-center gap-1 text-(--icon-muted)">
          <NaviToolbarButton label="显示边栏">
            <PanelLeft className="h-3.5 w-3.5" />
          </NaviToolbarButton>
          <NaviToolbarButton label="后退">
            <ArrowLeft className="h-3.5 w-3.5" />
          </NaviToolbarButton>
          <NaviToolbarButton label="前进">
            <ArrowRight className="h-3.5 w-3.5" />
          </NaviToolbarButton>
          <NaviToolbarButton label={event.phase === "running" ? "正在加载" : "重新载入"}>
            {event.phase === "running"
              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
              : <RefreshCw className="h-3.5 w-3.5" />}
          </NaviToolbarButton>
        </div>
        <div className="flex min-w-0 flex-1 items-center gap-2 rounded-[9px] border border-(--divider-subtle-color) bg-white/88 px-2.5 py-1.5 text-[11px] text-(--text-default) shadow-[inset_0_1px_0_rgba(255,255,255,0.76)]">
          <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[color:var(--success)]" />
          <span className="min-w-0 flex-1 truncate font-medium">{displayUrl}</span>
          <NaviLoadDot sourceLabel={sourceLabel} status={status} />
        </div>
        <NaviToolbarButton label="共享">
          <Share2 className="h-3.5 w-3.5" />
        </NaviToolbarButton>
        <NaviToolbarButton label="新建标签页">
          <Plus className="h-3.5 w-3.5" />
        </NaviToolbarButton>
        <NaviToolbarButton label="在浏览器中打开">
          <ExternalLink className="h-3.5 w-3.5" />
        </NaviToolbarButton>
      </div>
    </div>
  );
}

function NaviToolbarButton({ children, label }: { children: ReactNode; label: string }) {
  return (
    <button
      aria-label={label}
      className="grid h-6 w-6 place-items-center rounded-md border border-(--divider-subtle-color) bg-white/64 transition hover:bg-white focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.32)]"
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}

function NaviLoadDot({
  sourceLabel,
  status,
}: {
  sourceLabel: string;
  status: { label: string; tone: "loading" | "ready" | "error" | "idle" };
}) {
  return (
    <span
      aria-label={`${status.label} · ${sourceLabel}`}
      className={cn(
        "grid h-4 w-4 shrink-0 place-items-center rounded-full",
        status.tone === "loading" && "text-[color:var(--primary)]",
        status.tone === "ready" && "text-[color:var(--success)]",
        status.tone === "error" && "text-[color:var(--destructive)]",
        status.tone === "idle" && "text-(--icon-muted)",
      )}
      title={`${status.label} · ${sourceLabel}`}
    >
      {status.tone === "loading" ? <Loader2 className="h-3 w-3 animate-spin" /> : null}
      {status.tone === "error" ? <AlertTriangle className="h-3 w-3" /> : null}
      {status.tone !== "loading" && status.tone !== "error" ? (
        <span className="h-1.5 w-1.5 rounded-full bg-current" />
      ) : null}
    </span>
  );
}

function BrowserViewport({
  event,
  iframeUrl,
  lines,
  pageKind,
  preview,
  query,
  srcdoc,
  target,
}: {
  event: NexusOperationEvent;
  iframeUrl: string | null;
  lines: string[];
  pageKind: "embedded" | "workspace" | "web" | "search" | "start";
  preview: unknown;
  query: string;
  srcdoc: string | null;
  target?: string | null;
}) {
  if (srcdoc) {
    return (
      <HtmlFilePreview
        content={srcdoc}
        isStreaming={event.phase === "running"}
        title={target ?? query}
      />
    );
  }

  if (iframeUrl) {
    return (
      <BrowserIframeViewport
        sourceUrl={iframeUrl}
        title={target ?? query}
      />
    );
  }

  if (pageKind === "start") {
    return <NaviStartPage event={event} />;
  }

  if (isWebFetchEvent(event)) {
    return (
      <BrowserReaderPage
        event={event}
        lines={lines}
        preview={preview}
        query={query}
      />
    );
  }

  return <BrowserSearchResults event={event} lines={lines} query={query} />;
}

function BrowserIframeViewport({
  sourceUrl,
  title,
}: {
  sourceUrl: string;
  title: string;
}) {
  return (
    <iframe
      className="min-h-0 flex-1 bg-white"
      sandbox="allow-downloads allow-forms allow-modals allow-popups allow-scripts"
      src={sourceUrl}
      title={title}
    />
  );
}

function NaviStartPage({
  event,
}: {
  event: NexusOperationEvent;
}) {
  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[radial-gradient(70%_44%_at_50%_0%,rgba(91,114,255,0.06),transparent_72%),#fbfcfe]">
      <div className="mx-auto flex min-h-full max-w-[860px] flex-col items-center px-6 py-10 text-center">
        <div className="grid h-16 w-16 place-items-center rounded-[20px] bg-white/84 text-[color:var(--primary)] shadow-[0_20px_48px_rgba(18,28,42,0.10),inset_0_1px_0_rgba(255,255,255,0.86)]">
          <Globe2 className="h-7 w-7" />
        </div>
        <div className="mt-6 flex w-full max-w-[640px] min-w-0 items-center gap-2 rounded-[18px] border border-(--divider-subtle-color) bg-white px-4 py-3 text-[13px] text-(--text-strong) shadow-[0_16px_42px_rgba(18,28,42,0.08),inset_0_1px_0_rgba(255,255,255,0.82)]">
          <Search className="h-4 w-4 shrink-0 text-(--icon-muted)" />
          <span className="min-w-0 flex-1 text-left text-(--text-soft)">搜索或输入网站名称</span>
        </div>
        <p className="mt-3 text-[11px] text-(--text-soft)">
          Navi 起始页 · Nexus 待命 · {PHASE_LABEL[event.phase]}
        </p>

        <div className="mt-8 grid w-full grid-cols-3 gap-2 max-md:grid-cols-1">
          <NaviSummaryTile label="收藏" value="工作区预览" />
          <NaviSummaryTile label="阅读列表" value="空" />
          <NaviSummaryTile label="隐私报告" value="已启用" />
        </div>

        <div className="mt-4 grid w-full grid-cols-3 gap-3 max-md:grid-cols-1">
          {["工作区", "本地预览", "执行记录"].map((label) => (
            <div
              className="rounded-[16px] border border-(--divider-subtle-color) bg-white/68 px-4 py-4 text-left shadow-[0_18px_54px_rgba(18,28,42,0.06)]"
              key={label}
            >
              <div className="mb-3 grid h-9 w-9 place-items-center rounded-[12px] bg-[rgba(91,114,255,0.10)] text-[color:var(--primary)]">
                <BookOpen className="h-4 w-4" />
              </div>
              <p className="text-[12px] font-black text-(--text-strong)">{label}</p>
              <p className="mt-1 text-[10.5px] leading-4 text-(--text-soft)">等待 Nexus 打开页面</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}


function BrowserSearchResults({
  event,
  lines,
  query,
}: {
  event: NexusOperationEvent;
  lines: string[];
  query: string;
}) {
  const result_items = buildBrowserResultItems({ event, lines, query });

  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-white">
      <div className="min-h-full px-8 py-7">
        <div className="max-w-[760px]">
          <div className="mb-7">
            <div className="flex min-w-0 items-center gap-2 rounded-full border border-(--divider-subtle-color) bg-white px-4 py-2.5 text-[14px] text-(--text-strong) shadow-[0_3px_16px_rgba(18,28,42,0.08)]">
              <Search className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0 flex-1 truncate">{query}</span>
              {event.phase === "running" ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-[color:var(--primary)]" /> : null}
            </div>
            {event.phase === "running" ? (
              <div className="operation-web-loading mt-3 h-1 max-w-[420px] overflow-hidden rounded-full bg-[rgba(91,114,255,0.10)]" />
            ) : null}
          </div>

          <div className="space-y-6">
            {result_items.map((item) => (
              <article
                className="grid grid-cols-[minmax(0,1fr)_auto] gap-3"
                key={`${item.kind}:${item.url ?? item.title}:${item.snippet}`}
              >
                <div className="min-w-0">
                  {item.url ? (
                    <>
                      <p className="truncate text-[12px] leading-5 text-[#137333]">{item.url}</p>
                      <h3 className="mt-0.5 line-clamp-2 text-[18px] font-medium leading-6 tracking-normal text-[#1a0dab]">
                        {item.title}
                      </h3>
                    </>
                  ) : null}
                  <p className={cn("line-clamp-3 text-[#4d5156]", item.url ? "mt-1 text-[13px] leading-5" : "text-[13px] leading-5")}>
                    {item.snippet}
                  </p>
                </div>
                {item.url ? <ExternalLink className="mt-6 h-4 w-4 shrink-0 text-(--icon-muted)" /> : null}
              </article>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function BrowserReaderPage({
  event,
  lines,
  preview,
  query,
}: {
  event: NexusOperationEvent;
  lines: string[];
  preview: unknown;
  query: string;
}) {
  const url = readBrowserInputString(event, ["url", "uri", "link"]) ?? (looksLikeUrl(query) ? query : null);
  const prompt = readBrowserInputString(event, ["prompt", "question", "query"]) ?? event.summary ?? "";
  const paragraphs = buildBrowserReaderParagraphs({
    fallback: event.summary ?? prompt,
    lines,
    markers: [event.summary, prompt],
    preview,
  });
  const highlighted_count = paragraphs.filter((paragraph) => paragraph.highlighted).length;
  const origin = url ? formatBrowserOrigin(url) : formatBrowserOrigin(query);
  const title = event.title && event.title !== "抓取网页" ? event.title : origin;

  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[linear-gradient(180deg,#fbfcfe,#f3f6fb)]">
      <article className="mx-auto min-h-full max-w-[820px] px-7 py-7">
        <header className="border-b border-(--divider-subtle-color) pb-5">
          <div className="mb-4 flex items-center gap-2 text-[11px] font-semibold text-(--text-soft)">
            <span className="grid h-7 w-7 place-items-center rounded-[10px] bg-white text-[color:var(--primary)] shadow-[inset_0_1px_0_rgba(255,255,255,0.82)]">
              <FileText className="h-3.5 w-3.5" />
            </span>
            <span className="truncate">{url ?? query}</span>
          </div>
          <h1 className="text-[22px] font-black leading-7 tracking-normal text-(--text-strong)">
            {title}
          </h1>
          {prompt ? (
            <p className="mt-3 rounded-[12px] border border-(--divider-subtle-color) bg-white/72 px-3 py-2 text-[12px] leading-5 text-(--text-default)">
              {prompt}
            </p>
          ) : null}
          <div className="mt-4 flex flex-wrap gap-2 text-[10px] font-black text-(--text-soft)">
            <span className="rounded-full bg-white/72 px-2.5 py-1">Reader</span>
            <span className="rounded-full bg-white/72 px-2.5 py-1">{PHASE_LABEL[event.phase]}</span>
            <span className="rounded-full bg-white/72 px-2.5 py-1">{paragraphs.length} 段</span>
            {highlighted_count > 0 ? (
              <span className="rounded-full bg-[rgba(91,114,255,0.10)] px-2.5 py-1 text-[color:var(--primary)]">
                命中 {highlighted_count}
              </span>
            ) : null}
          </div>
        </header>

        <div className="space-y-4 py-5 text-[13px] leading-6 text-(--text-default)">
          {paragraphs.map((paragraph, index) => (
            <p
              className={cn(
                "whitespace-pre-wrap break-words rounded-[12px] px-3 py-2",
                paragraph.highlighted
                  ? "border border-[rgba(91,114,255,0.18)] bg-[rgba(91,114,255,0.07)] text-(--text-strong)"
                  : "border border-transparent",
              )}
              key={`${index}:${paragraph.text.slice(0, 20)}`}
            >
              {paragraph.text}
            </p>
          ))}
        </div>
      </article>
    </div>
  );
}

function NaviSummaryTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[12px] border border-(--divider-subtle-color) bg-white/66 px-3 py-2 text-left shadow-[0_10px_26px_rgba(18,28,42,0.045)]">
      <p className="text-[9px] font-black text-(--text-soft)">{label}</p>
      <p className="mt-0.5 truncate text-[11px] font-black text-(--text-strong)">{value}</p>
    </div>
  );
}
