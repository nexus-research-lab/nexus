/**
 * INPUT: Navi tabs, active page state, and navigation commands.
 * OUTPUT: Browser tabs, address bar, history controls, and Reader control.
 * POS: Chrome-only view; it does not interpret tool events or render page content.
 */
import {
  ArrowLeft,
  ArrowRight,
  BookOpen,
  FileCode2,
  Globe2,
  Loader2,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  X,
} from "lucide-react";
import type { FormEvent, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

import type { BrowserPageSnapshot } from "./browser-page-model";
import type { BrowserTabState } from "./browser-navigation-model";

export function BrowserChrome({
  activePage,
  activeTabId,
  addressDraft,
  canGoBack,
  canGoForward,
  isLoading,
  onAddressChange,
  onAddressSubmit,
  onCloseTab,
  onGoBack,
  onGoForward,
  onNewTab,
  onReload,
  onSelectTab,
  onToggleReader,
  tabs,
}: {
  activePage: BrowserPageSnapshot;
  activeTabId: string;
  addressDraft: string;
  canGoBack: boolean;
  canGoForward: boolean;
  isLoading: boolean;
  onAddressChange: (value: string) => void;
  onAddressSubmit: (value: string) => void;
  onCloseTab: (tab_id: string) => void;
  onGoBack: () => void;
  onGoForward: () => void;
  onNewTab: () => void;
  onReload: () => void;
  onSelectTab: (tab_id: string) => void;
  onToggleReader: () => void;
  tabs: BrowserTabState[];
}) {
  const submit_address = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onAddressSubmit(addressDraft);
  };

  return (
    <header className="shrink-0 border-b border-[#dce2ea] bg-[rgba(244,247,251,0.96)] text-[#202733] shadow-[0_1px_0_rgba(255,255,255,0.9)]">
      <div
        aria-label="标签页"
        className="soft-scrollbar flex min-w-0 items-end gap-1 overflow-x-auto px-2.5 pt-1.5"
        role="tablist"
      >
        {tabs.map((tab) => {
          const page = tab.pages[tab.current_index] ?? tab.pages[0];
          const active = tab.id === activeTabId;
          return page ? (
            <div
              className={cn(
                "group flex h-7 min-w-[116px] max-w-[210px] items-center rounded-t-[8px] border border-b-0",
                active
                  ? "border-[#dce2ea] bg-white shadow-[0_-1px_0_rgba(255,255,255,0.9)]"
                  : "border-transparent bg-transparent text-[#687180] hover:bg-white/56",
              )}
              key={tab.id}
            >
              <button
                aria-selected={active}
                className="flex min-w-0 flex-1 items-center gap-1.5 px-2 py-1 text-left focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-[#8192ff]"
                onClick={() => onSelectTab(tab.id)}
                role="tab"
                type="button"
              >
                <BrowserPageIcon kind={page.kind} />
                <span className="min-w-0 flex-1 truncate text-[10px] font-semibold">{page.tab_title}</span>
                {page.source === "agent" ? <span className="h-1.5 w-1.5 shrink-0 rounded-full bg-[#39b980]" title="Agent 页面" /> : null}
              </button>
              <button
                aria-label={`关闭 ${page.tab_title}`}
                className="mr-1 grid h-4 w-4 shrink-0 place-items-center rounded text-[#7a8390] opacity-0 transition hover:bg-[#e7ebf1] hover:text-[#27303b] focus-visible:opacity-100 focus-visible:outline-none group-hover:opacity-100"
                onClick={() => onCloseTab(tab.id)}
                title="关闭标签页"
                type="button"
              >
                <X className="h-3 w-3" />
              </button>
            </div>
          ) : null;
        })}
        <BrowserToolbarButton label="新建标签页" onClick={onNewTab} subtle>
          <Plus className="h-3.5 w-3.5" />
        </BrowserToolbarButton>
      </div>

      <div className="flex min-w-0 items-center gap-1.5 bg-white px-2.5 py-1.5">
        <BrowserToolbarButton disabled={!canGoBack} label="后退" onClick={onGoBack}>
          <ArrowLeft className="h-3.5 w-3.5" />
        </BrowserToolbarButton>
        <BrowserToolbarButton disabled={!canGoForward} label="前进" onClick={onGoForward}>
          <ArrowRight className="h-3.5 w-3.5" />
        </BrowserToolbarButton>
        <BrowserToolbarButton label="重新载入" onClick={onReload}>
          {isLoading
            ? <Loader2 className="h-3.5 w-3.5 animate-spin" />
            : <RefreshCw className="h-3.5 w-3.5" />}
        </BrowserToolbarButton>

        <form className="min-w-0 flex-1" onSubmit={submit_address}>
          <label className="flex min-w-0 items-center gap-2 rounded-[8px] border border-[#dfe4eb] bg-[#f3f5f8] px-2.5 py-1.5 shadow-[inset_0_1px_1px_rgba(26,35,48,0.03)] focus-within:border-[#a9b4ff] focus-within:bg-white focus-within:ring-2 focus-within:ring-[#8797ff]/15">
            <span className="sr-only">网址</span>
            {addressDraft.startsWith("https://")
              ? <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-[#2fa773]" />
              : addressDraft.startsWith("navi://")
                ? <Search className="h-3.5 w-3.5 shrink-0 text-[#77808d]" />
                : <Globe2 className="h-3.5 w-3.5 shrink-0 text-[#77808d]" />}
            <input
              autoCapitalize="none"
              autoComplete="off"
              className="min-w-0 flex-1 bg-transparent text-[11px] font-medium text-[#303846] outline-none placeholder:text-[#929aa6]"
              onChange={(event) => onAddressChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  onAddressSubmit(addressDraft);
                }
              }}
              placeholder="输入网址或本轮搜索词"
              spellCheck={false}
              value={addressDraft}
            />
            <BrowserLoadState isLoading={isLoading} page={activePage} />
          </label>
        </form>

        {activePage.reader ? (
          <BrowserToolbarButton
            active={activePage.presentation === "reader"}
            label={activePage.presentation === "reader" ? "显示网页" : "阅读模式"}
            onClick={onToggleReader}
          >
            <BookOpen className="h-3.5 w-3.5" />
          </BrowserToolbarButton>
        ) : null}
      </div>
    </header>
  );
}

function BrowserToolbarButton({
  active = false,
  children,
  disabled = false,
  label,
  onClick,
  subtle = false,
}: {
  active?: boolean;
  children: ReactNode;
  disabled?: boolean;
  label: string;
  onClick: () => void;
  subtle?: boolean;
}) {
  return (
    <button
      aria-label={label}
      className={cn(
        "grid h-6 w-6 shrink-0 place-items-center rounded-[6px] text-[#596271] transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[#8192ff]/45 disabled:cursor-default disabled:opacity-30",
        active ? "bg-[#e4e8ff] text-[#5367f1]" : "hover:bg-[#edf0f4]",
        subtle && "mb-0.5 text-[#7a8390]",
      )}
      disabled={disabled}
      onClick={onClick}
      title={label}
      type="button"
    >
      {children}
    </button>
  );
}

function BrowserLoadState({
  isLoading,
  page,
}: {
  isLoading: boolean;
  page: BrowserPageSnapshot;
}) {
  return (
    <span
      aria-label={isLoading ? "正在载入" : page.status.label}
      className={cn(
        "grid h-3.5 w-3.5 shrink-0 place-items-center",
        isLoading && "text-[#6677f4]",
        !isLoading && page.status.tone === "error" && "text-[#d8565d]",
        !isLoading && page.status.tone !== "error" && "text-[#38aa78]",
      )}
      title={`${isLoading ? "正在载入" : page.status.label} · ${page.source_label}`}
    >
      {isLoading
        ? <Loader2 className="h-3 w-3 animate-spin" />
        : <span className="h-1.5 w-1.5 rounded-full bg-current" />}
    </span>
  );
}

function BrowserPageIcon({ kind }: { kind: BrowserPageSnapshot["kind"] }) {
  if (kind === "search") {
    return <Search className="h-3 w-3 shrink-0 text-[#6875e8]" />;
  }
  if (kind === "embedded" || kind === "workspace") {
    return <FileCode2 className="h-3 w-3 shrink-0 text-[#31a875]" />;
  }
  return <Globe2 className="h-3 w-3 shrink-0 text-[#7b8492]" />;
}
