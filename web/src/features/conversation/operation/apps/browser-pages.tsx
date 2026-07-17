/**
 * INPUT: One Navi page snapshot and navigation callbacks.
 * OUTPUT: Interactive live, workspace HTML, search, start, or Reader page content.
 * POS: Page viewport renderer; browser session ownership stays in navigation-model.
 */
import {
  ArrowUpRight,
  FileText,
  Globe2,
  Loader2,
  Search,
} from "lucide-react";
import type { FormEvent } from "react";
import { useEffect, useRef, useState } from "react";

import { HtmlFilePreview } from "@/features/conversation/shared/editor/media/html-file-preview";
import { cn } from "@/shared/ui/class-name";

import type { BrowserPageSnapshot } from "./browser-page-model";
import { readBrowserPageBridgeMessage } from "./browser-page-bridge";

export function BrowserPageViewport({
  onFrameLoading,
  onFrameError,
  onNavigate,
  page,
}: {
  onFrameLoading: (page_id: string, loading: boolean) => void;
  onFrameError: (page_id: string, failed: boolean) => void;
  onNavigate: (target: string) => void;
  page: BrowserPageSnapshot;
}) {
  if (page.presentation === "reader" && page.reader) {
    return <BrowserReaderPage page={page} />;
  }
  if (page.srcdoc) {
    return (
      <HtmlFilePreview
        content={page.srcdoc}
        isStreaming={page.event?.phase === "running"}
        presentation="browser"
        title={page.tab_title}
      />
    );
  }
  if (page.iframe_url) {
    return (
      <BrowserLivePage
        onFrameError={onFrameError}
        onFrameLoading={onFrameLoading}
        onNavigate={onNavigate}
        page={page}
      />
    );
  }
  if (page.kind === "start") {
    return <BrowserStartPage onNavigate={onNavigate} />;
  }
  if (page.kind === "search") {
    return <BrowserSearchPage onNavigate={onNavigate} page={page} />;
  }
  return <BrowserReaderPage page={page} />;
}

function BrowserLivePage({
  onFrameError,
  onFrameLoading,
  onNavigate,
  page,
}: {
  onFrameError: (page_id: string, failed: boolean) => void;
  onFrameLoading: (page_id: string, loading: boolean) => void;
  onNavigate: (target: string) => void;
  page: BrowserPageSnapshot;
}) {
  const frame_key = `${page.id}:${page.reload_key}:${page.iframe_url}`;
  const frame_ref = useRef<HTMLIFrameElement>(null);

  useEffect(() => {
    onFrameError(page.id, false);
    onFrameLoading(page.id, true);
    return () => onFrameLoading(page.id, false);
  }, [frame_key, onFrameError, onFrameLoading, page.id]);

  useEffect(() => {
    const receive = (event: MessageEvent<unknown>) => {
      if (event.source !== frame_ref.current?.contentWindow) {
        return;
      }
      const message = readBrowserPageBridgeMessage(event.data);
      if (message?.type === "navigate") {
        onNavigate(message.url);
      } else if (message?.type === "load-error") {
        onFrameError(page.id, true);
        onFrameLoading(page.id, false);
      }
    };
    window.addEventListener("message", receive);
    return () => window.removeEventListener("message", receive);
  }, [onFrameError, onFrameLoading, onNavigate, page.id]);

  return (
    <div className="relative min-h-0 flex-1 bg-white">
      <iframe
        className="h-full min-h-0 w-full bg-white"
        key={frame_key}
        onError={() => {
          onFrameError(page.id, true);
          onFrameLoading(page.id, false);
        }}
        onLoad={() => onFrameLoading(page.id, false)}
        ref={frame_ref}
        referrerPolicy="no-referrer-when-downgrade"
        sandbox="allow-downloads allow-forms allow-modals allow-pointer-lock allow-scripts"
        src={page.iframe_url ?? undefined}
        title={page.tab_title}
      />
    </div>
  );
}

function BrowserStartPage({ onNavigate }: { onNavigate: (target: string) => void }) {
  const [value, set_value] = useState("");
  const submit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (value.trim()) {
      onNavigate(value);
    }
  };

  return (
    <div className="min-h-0 flex-1 overflow-auto bg-[#f8fafc]">
      <div className="mx-auto flex min-h-full max-w-[720px] flex-col items-center justify-center px-8 pb-20 pt-12">
        <div className="grid h-14 w-14 place-items-center rounded-[15px] bg-[#232a34] text-white shadow-[0_18px_42px_rgba(26,34,47,0.18)]">
          <Globe2 className="h-6 w-6" />
        </div>
        <p className="mt-4 text-[20px] font-semibold tracking-normal text-[#202732]">Navi</p>
        <form className="mt-7 w-full" onSubmit={submit}>
          <label className="flex w-full items-center gap-3 rounded-[14px] border border-[#dfe4ea] bg-white px-4 py-3 shadow-[0_12px_32px_rgba(27,36,50,0.08)] focus-within:border-[#a9b4ff] focus-within:ring-3 focus-within:ring-[#8394ff]/10">
            <Search className="h-4 w-4 shrink-0 text-[#7c8592]" />
            <span className="sr-only">输入网址</span>
            <input
              autoCapitalize="none"
              autoComplete="off"
              className="min-w-0 flex-1 bg-transparent text-[13px] text-[#252d39] outline-none placeholder:text-[#949ca7]"
              onChange={(event) => set_value(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  if (value.trim()) {
                    onNavigate(value);
                  }
                }
              }}
              placeholder="输入网址"
              spellCheck={false}
              value={value}
            />
          </label>
        </form>
      </div>
    </div>
  );
}

function BrowserSearchPage({
  onNavigate,
  page,
}: {
  onNavigate: (target: string) => void;
  page: BrowserPageSnapshot;
}) {
  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-white">
      <main className="mx-auto min-h-full max-w-[940px] px-8 py-7">
        <header className="mb-7 max-w-[720px] border-b border-[#edf0f3] pb-5">
          <div className="flex min-w-0 items-center gap-2.5 rounded-[13px] border border-[#dfe3e8] bg-white px-4 py-2.5 shadow-[0_3px_14px_rgba(22,32,46,0.08)]">
            <Search className="h-4 w-4 shrink-0 text-[#777f8b]" />
            <h1 className="min-w-0 flex-1 truncate text-[14px] font-normal text-[#303642]">{page.query}</h1>
            {page.event?.phase === "running" ? <Loader2 className="h-4 w-4 animate-spin text-[#6577ee]" /> : null}
          </div>
        </header>

        {page.results.length ? (
          <div className="max-w-[720px] space-y-7">
            {page.results.map((item, index) => (
              <article key={`${item.kind}:${item.url ?? item.title}:${index}`}>
                {item.url ? (
                  <button
                    className="group block max-w-full text-left focus-visible:outline-none"
                    onClick={() => onNavigate(item.url ?? "")}
                    type="button"
                  >
                    <span className="flex min-w-0 items-center gap-2 text-[11px] text-[#4e5967]">
                      <span className="grid h-6 w-6 shrink-0 place-items-center rounded-full bg-[#f0f2f5] text-[#687381]">
                        <Globe2 className="h-3 w-3" />
                      </span>
                      <span className="min-w-0 truncate">{format_result_origin(item.url)}</span>
                    </span>
                    <span className="mt-1 flex min-w-0 items-start gap-1.5 text-[#1a46a1] group-hover:underline">
                      <span className="line-clamp-2 text-[17px] font-medium leading-6">{item.title}</span>
                      <ArrowUpRight className="mt-1.5 h-3.5 w-3.5 shrink-0 opacity-0 transition group-hover:opacity-70" />
                    </span>
                  </button>
                ) : (
                  <h2 className="text-[14px] font-semibold text-[#303742]">{item.title}</h2>
                )}
                <p className="mt-1 line-clamp-3 text-[12px] leading-5 text-[#565e69]">{item.snippet}</p>
              </article>
            ))}
          </div>
        ) : (
          <div className="max-w-[720px] border-t border-[#edf0f3] py-10 text-[12px] text-[#747d89]">
            本轮执行没有这项搜索记录。
          </div>
        )}
      </main>
    </div>
  );
}

function BrowserReaderPage({ page }: { page: BrowserPageSnapshot }) {
  const reader = page.reader;
  const fallback = page.event?.summary ?? page.query;
  const paragraphs = reader?.paragraphs.length
    ? reader.paragraphs
    : [{ highlighted: false, text: fallback }];
  const title = reader?.origin || page.tab_title;

  return (
    <div className="soft-scrollbar min-h-0 flex-1 overflow-auto bg-[#f7f8fa]">
      <article className="mx-auto min-h-full max-w-[780px] bg-white px-9 py-8 shadow-[0_0_0_1px_rgba(27,37,50,0.04)]">
        <header className="border-b border-[#e9edf1] pb-5">
          <div className="flex min-w-0 items-center gap-2 text-[10px] font-medium text-[#6e7784]">
            <FileText className="h-3.5 w-3.5 shrink-0 text-[#6577ee]" />
            <span className="truncate">{reader?.url ?? page.address}</span>
          </div>
          <h1 className="mt-3 text-[23px] font-semibold leading-7 tracking-normal text-[#202733]">{title}</h1>
          {reader?.prompt ? (
            <p className="mt-3 max-w-[680px] text-[12px] leading-5 text-[#68717e]">{reader.prompt}</p>
          ) : null}
        </header>

        <div className="space-y-4 py-6 text-[13px] leading-6 text-[#38414d]">
          {paragraphs.map((paragraph, index) => (
            <p
              className={cn(
                "whitespace-pre-wrap break-words border-l-2 py-0.5 pl-4",
                paragraph.highlighted
                  ? "border-[#7181ef] bg-[#f6f7ff] pr-3 text-[#252d3a]"
                  : "border-transparent",
              )}
              key={`${index}:${paragraph.text.slice(0, 24)}`}
            >
              {paragraph.text}
            </p>
          ))}
        </div>
      </article>
    </div>
  );
}

function format_result_origin(value: string): string {
  try {
    const url = new URL(value);
    return `${url.hostname}${url.pathname === "/" ? "" : url.pathname}`;
  } catch {
    return value;
  }
}
