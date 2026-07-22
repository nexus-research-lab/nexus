/**
 * INPUT: One Library app session projected from Skill/knowledge tool events.
 * OUTPUT: A searchable, persistent macOS-like Library window with truthful Markdown content.
 * POS: Operation Stage Library UI; content extraction remains in library-session-model.
 */
import { useEffect, useMemo, useState } from "react";
import { BookOpen, CircleAlert, Loader2, Search } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";

import type { NexusOperationEvent } from "../operation-types";
import {
  buildLibrarySessionView,
  filterLibraryEntries,
  type LibraryEntry,
} from "./library-session-model";

export function LibrarySurface({
  event,
  relatedEvents,
}: {
  event: NexusOperationEvent;
  relatedEvents: NexusOperationEvent[];
}) {
  const view = useMemo(
    () => buildLibrarySessionView({ event, relatedEvents }),
    [event, relatedEvents],
  );
  const [query, setQuery] = useState("");
  const [selectedEntryId, setSelectedEntryId] = useState<string | null>(view.active_entry_id);

  useEffect(() => {
    setSelectedEntryId(view.active_entry_id);
  }, [view.active_entry_id]);

  const visible_entries = useMemo(
    () => filterLibraryEntries(view.entries, query),
    [query, view.entries],
  );
  const selected_entry = view.entries.find((entry) => entry.id === selectedEntryId)
    ?? visible_entries[0]
    ?? view.entries[0]
    ?? null;

  return (
    <div className="grid h-full min-h-[300px] grid-cols-[210px_minmax(0,1fr)] overflow-hidden bg-[#f6f7f9] text-(--text-default) max-md:grid-cols-1">
      <aside className="flex min-h-0 flex-col border-r border-(--divider-subtle-color) bg-[#eef1f5]/94 max-md:hidden">
        <div className="border-b border-(--divider-subtle-color) px-3 py-3">
          <div className="flex items-center gap-2 px-1">
            <span className="grid h-8 w-8 place-items-center rounded-[9px] bg-[#25344a] text-white shadow-[0_8px_20px_rgba(24,38,58,0.14)]">
              <BookOpen className="h-4 w-4" />
            </span>
            <div className="min-w-0">
              <p className="truncate text-[12px] font-black text-(--text-strong)">Library</p>
              <p className="truncate text-[9.5px] text-(--text-soft)">本轮上下文 · {view.entries.length}</p>
            </div>
          </div>

          <label className="mt-3 flex h-8 items-center gap-2 rounded-[8px] border border-(--divider-subtle-color) bg-white/72 px-2.5 text-(--text-soft)">
            <Search className="h-3.5 w-3.5 shrink-0" />
            <input
              aria-label="搜索 Library"
              className="min-w-0 flex-1 bg-transparent text-[10.5px] font-semibold text-(--text-strong) outline-none placeholder:text-(--text-soft)"
              onChange={(input_event) => setQuery(input_event.target.value)}
              placeholder="搜索"
              value={query}
            />
          </label>
        </div>

        <div className="soft-scrollbar min-h-0 flex-1 overflow-auto p-2">
          {visible_entries.map((entry) => (
            <LibraryEntryButton
              active={entry.id === selected_entry?.id}
              entry={entry}
              key={entry.id}
              onSelect={() => setSelectedEntryId(entry.id)}
            />
          ))}
          {visible_entries.length === 0 ? (
            <p className="px-2 py-4 text-center text-[10px] font-semibold text-(--text-soft)">没有匹配条目</p>
          ) : null}
        </div>
      </aside>

      <section className="flex min-h-0 min-w-0 flex-col bg-white/86">
        {selected_entry ? (
          <>
            <header className="flex min-w-0 items-center justify-between gap-3 border-b border-(--divider-subtle-color) bg-white/80 px-4 py-2.5">
              <div className="min-w-0">
                <p className="truncate text-[13px] font-black text-(--text-strong)">{selected_entry.name}</p>
                <p className="truncate text-[9.5px] font-semibold text-(--text-soft)">
                  {selected_entry.event.tool_name ?? "Skill"}
                </p>
              </div>
              <LibraryPhase phase={selected_entry.phase} />
            </header>
            <div className="soft-scrollbar min-h-0 flex-1 overflow-auto px-5 py-4">
              {selected_entry.description ? (
                <p className="mb-4 border-b border-(--divider-subtle-color) pb-3 text-[11px] font-semibold leading-5 text-(--text-soft)">
                  {selected_entry.description}
                </p>
              ) : null}
              <UiMarkdownContent
                className="text-[12px] leading-6"
                content={selected_entry.content}
                isStreaming={selected_entry.phase === "running"}
                mermaidShowHeader={false}
                workspaceAgentId={selected_entry.event.agent_id}
              />
            </div>
          </>
        ) : (
          <div className="grid h-full place-items-center text-(--text-soft)">
            <div className="text-center">
              <BookOpen className="mx-auto h-7 w-7" />
              <p className="mt-2 text-[11px] font-black">Library 尚无条目</p>
            </div>
          </div>
        )}
      </section>
    </div>
  );
}

function LibraryEntryButton({
  active,
  entry,
  onSelect,
}: {
  active: boolean;
  entry: LibraryEntry;
  onSelect: () => void;
}) {
  return (
    <button
      className={cn(
        "mb-1 flex w-full min-w-0 items-center gap-2 rounded-[8px] px-2 py-2 text-left transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[rgba(91,114,255,0.30)]",
        active ? "bg-white text-(--text-strong) shadow-[0_5px_16px_rgba(18,28,42,0.06)]" : "text-(--text-soft) hover:bg-white/58",
      )}
      onClick={onSelect}
      type="button"
    >
      <BookOpen className="h-3.5 w-3.5 shrink-0" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-[10.5px] font-black">{entry.name}</span>
        <span className="mt-0.5 block truncate text-[9px] font-semibold opacity-72">
          {entry.description ?? entry.event.tool_name ?? "Skill"}
        </span>
      </span>
    </button>
  );
}

function LibraryPhase({ phase }: { phase: NexusOperationEvent["phase"] }) {
  const running = phase === "running" || phase === "queued";
  const failed = phase === "error" || phase === "cancelled";
  const Icon = running ? Loader2 : failed ? CircleAlert : BookOpen;
  const label = running ? "读取中" : failed ? "未载入" : "已载入";
  return (
    <span className={cn(
      "inline-flex h-6 shrink-0 items-center gap-1.5 rounded-full px-2.5 text-[9.5px] font-black",
      failed
        ? "bg-[rgba(223,93,98,0.10)] text-[color:var(--destructive)]"
        : running
          ? "bg-[rgba(91,114,255,0.10)] text-[color:var(--primary)]"
          : "bg-[rgba(47,184,132,0.10)] text-[color:var(--success)]",
    )}>
      <Icon className={cn("h-3 w-3", running && "animate-spin")} />
      {label}
    </span>
  );
}
