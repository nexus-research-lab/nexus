import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  BookOpen,
  ExternalLink,
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

import { HtmlFilePreview } from "@/features/conversation/shared/editor/html-file-preview";
import { getWorkspaceFilePreviewUrl } from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";

import type {
  NexusOperationEvent,
  OperationPhase,
} from "../operation-types";
import { build_browser_result_items } from "./browser-result-items";
import { build_browser_session_view } from "./browser-session";

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
  const session = build_browser_session_view({
    event,
    preview,
    query,
    raw_url_builder: getWorkspaceFilePreviewUrl,
    target,
  });

  return (
    <div className="flex min-h-0 min-w-0 max-w-full flex-1 flex-col overflow-hidden bg-[#f7f9fc] shadow-[inset_0_1px_0_rgba(255,255,255,0.82)]">
      <BrowserChromeHeader
        display_url={session.display_url}
        event={event}
        source_label={session.source_label}
        status={session.status}
        tab_title={session.tab_title}
      />

      <BrowserViewport
        iframe_url={session.iframe_url}
        page_kind={session.page_kind}
        query={query}
        srcdoc={session.srcdoc}
        target={target}
        event={event}
        lines={lines}
      />
    </div>
  );
}

function BrowserChromeHeader({
  display_url,
  event,
  source_label,
  status,
  tab_title,
}: {
  display_url: string;
  event: NexusOperationEvent;
  source_label: string;
  status: { label: string; tone: "loading" | "ready" | "error" | "idle" };
  tab_title: string;
}) {
  return (
    <div className="border-b border-(--divider-subtle-color) bg-[rgba(248,250,253,0.88)]">
      <div className="flex min-w-0 items-end gap-1.5 px-3 pt-2">
        <div className="flex min-w-0 max-w-[52%] items-center gap-1.5 rounded-t-[10px] border border-b-0 border-(--divider-subtle-color) bg-white/72 px-3 py-1.5 text-[10px] font-bold text-(--text-strong)">
          <Globe2 className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" />
          <span className="truncate">{tab_title}</span>
        </div>
      </div>
      <div className="flex min-w-0 items-center gap-2 px-3 py-2">
        <div className="flex shrink-0 items-center gap-1 text-(--icon-muted)">
          <SafariToolbarButton label="显示边栏">
            <PanelLeft className="h-3.5 w-3.5" />
          </SafariToolbarButton>
          <SafariToolbarButton label="后退">
            <ArrowLeft className="h-3.5 w-3.5" />
          </SafariToolbarButton>
          <SafariToolbarButton label="前进">
            <ArrowRight className="h-3.5 w-3.5" />
          </SafariToolbarButton>
          <SafariToolbarButton label={event.phase === "running" ? "正在加载" : "重新载入"}>
            {event.phase === "running"
              ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
              : <RefreshCw className="h-3.5 w-3.5" />}
          </SafariToolbarButton>
        </div>
        <div className="flex min-w-0 flex-1 items-center gap-2 rounded-[9px] border border-(--divider-subtle-color) bg-white/88 px-2.5 py-1.5 text-[11px] text-(--text-default) shadow-[inset_0_1px_0_rgba(255,255,255,0.76)]">
          <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[color:var(--success)]" />
          <span className="min-w-0 flex-1 truncate font-medium">{display_url}</span>
          <SafariLoadDot source_label={source_label} status={status} />
        </div>
        <SafariToolbarButton label="共享">
          <Share2 className="h-3.5 w-3.5" />
        </SafariToolbarButton>
        <SafariToolbarButton label="新建标签页">
          <Plus className="h-3.5 w-3.5" />
        </SafariToolbarButton>
        <SafariToolbarButton label="在浏览器中打开">
          <ExternalLink className="h-3.5 w-3.5" />
        </SafariToolbarButton>
      </div>
    </div>
  );
}

function SafariToolbarButton({ children, label }: { children: ReactNode; label: string }) {
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

function SafariLoadDot({
  source_label,
  status,
}: {
  source_label: string;
  status: { label: string; tone: "loading" | "ready" | "error" | "idle" };
}) {
  return (
    <span
      aria-label={`${status.label} · ${source_label}`}
      className={cn(
        "grid h-4 w-4 shrink-0 place-items-center rounded-full",
        status.tone === "loading" && "text-[color:var(--primary)]",
        status.tone === "ready" && "text-[color:var(--success)]",
        status.tone === "error" && "text-[color:var(--destructive)]",
        status.tone === "idle" && "text-(--icon-muted)",
      )}
      title={`${status.label} · ${source_label}`}
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
  iframe_url,
  lines,
  page_kind,
  query,
  srcdoc,
  target,
}: {
  event: NexusOperationEvent;
  iframe_url: string | null;
  lines: string[];
  page_kind: "embedded" | "workspace" | "web" | "search" | "start";
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

  if (iframe_url) {
    return (
      <BrowserIframeViewport
        sourceUrl={iframe_url}
        title={target ?? query}
      />
    );
  }

  if (page_kind === "start") {
    return <SafariStartPage event={event} />;
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

function SafariStartPage({
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
          Safari 起始页 · Nexus 待命 · {PHASE_LABEL[event.phase]}
        </p>

        <div className="mt-8 grid w-full grid-cols-3 gap-2 max-md:grid-cols-1">
          <SafariSummaryTile label="收藏" value="工作区预览" />
          <SafariSummaryTile label="阅读列表" value="空" />
          <SafariSummaryTile label="隐私报告" value="已启用" />
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
  const result_items = build_browser_result_items({ event, lines, query });
  const has_link_results = result_items.some((item) => item.kind === "link");
  const result_label = has_link_results ? "搜索结果" : "工具返回摘要";

  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[radial-gradient(70%_42%_at_50%_0%,rgba(91,114,255,0.055),transparent_70%),#fbfcfe]">
      <div className="min-h-full px-6 py-6">
        <div className="mx-auto max-w-[860px]">
          <div className="mb-5 text-center">
            <div className="mx-auto mb-4 grid h-14 w-14 place-items-center rounded-[18px] bg-white/82 text-[color:var(--primary)] shadow-[0_18px_42px_rgba(18,28,42,0.10),inset_0_1px_0_rgba(255,255,255,0.84)]">
              <Globe2 className="h-6 w-6" />
            </div>
            <div className="mx-auto flex max-w-[640px] min-w-0 items-center gap-2 rounded-[18px] border border-(--divider-subtle-color) bg-white px-4 py-3 text-[13px] text-(--text-strong) shadow-[0_16px_42px_rgba(18,28,42,0.08),inset_0_1px_0_rgba(255,255,255,0.82)]">
              <Search className="h-4 w-4 shrink-0 text-(--icon-muted)" />
              <span className="min-w-0 flex-1 truncate">{query}</span>
              {event.phase === "running" ? <Loader2 className="h-4 w-4 shrink-0 animate-spin text-[color:var(--primary)]" /> : null}
            </div>
            <p className="mt-3 text-[11px] text-(--text-soft)">
              Safari {has_link_results ? "搜索" : "阅读"} · {PHASE_LABEL[event.phase]} · {result_items.length} 条记录
            </p>
            {event.phase === "running" ? (
              <div className="operation-web-loading mx-auto mt-3 h-1.5 max-w-[420px] overflow-hidden rounded-full bg-[rgba(91,114,255,0.10)]" />
            ) : null}
          </div>

          <div className="mb-3 grid grid-cols-3 gap-2 max-md:grid-cols-1">
            <SafariSummaryTile label="来源" value={format_browser_origin(query)} />
            <SafariSummaryTile label="状态" value={PHASE_LABEL[event.phase]} />
            <SafariSummaryTile label="记录" value={`${result_items.length} 条`} />
          </div>

          <div className="overflow-hidden rounded-[16px] border border-(--divider-subtle-color) bg-white/74 shadow-[0_18px_54px_rgba(18,28,42,0.075)]">
            <div className="grid grid-cols-[22px_minmax(0,1fr)_auto] items-center gap-2 border-b border-(--divider-subtle-color) bg-[#f4f6fa] px-4 py-2 text-[10px] font-black text-(--text-soft)">
              <BookOpen className="h-3.5 w-3.5" />
              <span>{result_label}</span>
              <span>Safari</span>
            </div>
            {result_items.map((item) => (
              <article
                className="grid grid-cols-[minmax(0,1fr)_auto] gap-3 border-b border-(--divider-subtle-color) px-4 py-3 last:border-b-0"
                key={`${item.kind}:${item.url ?? item.title}:${item.snippet}`}
              >
                <div className="min-w-0">
                  <p className="truncate text-[11px] font-semibold text-[color:var(--success)]">
                    {item.url ?? "工具输出摘要"}
                  </p>
                  <h3 className="mt-1 line-clamp-2 text-[14px] font-black tracking-normal text-(--text-strong)">
                    {item.title}
                  </h3>
                  <p className="mt-1.5 line-clamp-2 text-[12px] leading-5 text-(--text-default)">{item.snippet}</p>
                </div>
                {item.url ? <ExternalLink className="mt-1 h-4 w-4 shrink-0 text-(--icon-muted)" /> : null}
              </article>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function SafariSummaryTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[12px] border border-(--divider-subtle-color) bg-white/66 px-3 py-2 text-left shadow-[0_10px_26px_rgba(18,28,42,0.045)]">
      <p className="text-[9px] font-black text-(--text-soft)">{label}</p>
      <p className="mt-0.5 truncate text-[11px] font-black text-(--text-strong)">{value}</p>
    </div>
  );
}

function format_browser_origin(value: string): string {
  try {
    return new URL(value).hostname;
  } catch {
    return "工作区预览";
  }
}
