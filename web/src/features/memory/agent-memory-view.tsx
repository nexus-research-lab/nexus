"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  BookOpenText,
  Brain,
  Clock3,
  FileText,
  FolderKanban,
  History,
  Link2,
  LoaderCircle,
  MessageSquareWarning,
  RefreshCw,
  Search,
  UserRound,
} from "lucide-react";

import { getAgentMemorySnapshotApi } from "@/lib/api/memory-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiAgentAvatar } from "@/shared/ui/avatar";
import { UiIconButton } from "@/shared/ui/button";
import { UiSearchInput } from "@/shared/ui/form-control";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { Agent } from "@/types/agent/agent";
import type { MemoryDocument, MemoryDocumentType, MemorySnapshot } from "@/types/memory/memory";

import { MemoryDocumentPanel } from "./memory-document-panel";
import {
  formatMemoryModifiedTime,
  memoryAgeDays,
  memoryDocumentMatches,
  type MemoryFilter,
} from "./memory-utils";
import "./memory-view.css";

interface AgentMemoryViewProps {
  agent: Agent;
  agents?: Agent[];
  onAgentChange?: (agentId: string) => void;
}

const FILTERS: MemoryFilter[] = ["all", "user", "feedback", "project", "reference", "daily_log"];

export function AgentMemoryView({ agent, agents, onAgentChange }: AgentMemoryViewProps) {
  const { locale, t } = useI18n();
  const [snapshot, setSnapshot] = useState<MemorySnapshot | null>(null);
  const [selectedPath, setSelectedPath] = useState("");
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<MemoryFilter>("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [compactDocumentOpen, setCompactDocumentOpen] = useState(false);

  const refresh = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const nextSnapshot = await getAgentMemorySnapshotApi(agent.agent_id);
      setSnapshot(nextSnapshot);
      const documents = [nextSnapshot.index, ...nextSnapshot.documents].filter(Boolean) as MemoryDocument[];
      setSelectedPath((current) => documents.some((document) => document.path === current)
        ? current
        : documents[0]?.path ?? "");
    } catch (loadError) {
      setSnapshot(null);
      setSelectedPath("");
      setError(loadError instanceof Error ? loadError.message : t("capability.memory_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [agent.agent_id, t]);

  useEffect(() => {
    setCompactDocumentOpen(false);
    setQuery("");
    setFilter("all");
    void refresh();
  }, [agent.agent_id, refresh]);

  const allDocuments = useMemo(
    () => snapshot ? [snapshot.index, ...snapshot.documents].filter(Boolean) as MemoryDocument[] : [],
    [snapshot],
  );
  const selectedDocument = useMemo(
    () => allDocuments.find((document) => document.path === selectedPath) ?? null,
    [allDocuments, selectedPath],
  );
  const visibleDocuments = useMemo(
    () => (snapshot?.documents ?? []).filter((document) => memoryDocumentMatches(document, filter, query)),
    [filter, query, snapshot?.documents],
  );
  const indexVisible = Boolean(
    snapshot?.index && memoryDocumentMatches(snapshot.index, "index", query),
  );
  const counts = useMemo(() => memoryCounts(snapshot), [snapshot]);
  const latestDocument = snapshot?.documents[0] ?? snapshot?.index ?? null;

  const selectDocument = (path: string) => {
    if (!allDocuments.some((document) => document.path === path)) {
      return;
    }
    setSelectedPath(path);
    setCompactDocumentOpen(true);
  };

  const filterLabel = (value: MemoryFilter): string => {
    switch (value) {
      case "user": return t("capability.memory_type_user");
      case "feedback": return t("capability.memory_type_feedback");
      case "project": return t("capability.memory_type_project");
      case "reference": return t("capability.memory_type_reference");
      case "daily_log": return t("capability.memory_type_daily_log");
      default: return t("capability.memory_filter_all");
    }
  };

  return (
    <div
      className="nexus-memory-view flex min-h-0 min-w-0 flex-1 flex-col"
      data-document-open={compactDocumentOpen ? "true" : "false"}
    >
      <div className="nexus-memory-summary flex shrink-0 flex-wrap items-center gap-x-5 gap-y-2 border-b border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-raised-background)_52%,transparent)] px-4 py-3">
        {agents && onAgentChange ? (
          <div className="nexus-memory-agent-switcher flex min-w-[210px] max-w-[300px] flex-1 items-center gap-2.5">
            <UiAgentAvatar avatar={agent.avatar} name={agent.name} size="sm" />
            <UiSelectMenu
              ariaLabel={t("capability.memory_agent_aria")}
              buttonClassName="rounded-[8px]"
              onChange={onAgentChange}
              options={agents.map((item) => ({ value: item.agent_id, label: item.name }))}
              size="sm"
              value={agent.agent_id}
            />
          </div>
        ) : (
          <div className="flex min-w-0 items-center gap-2">
            <Brain className="h-4 w-4 text-(--icon-muted)" />
            <span className="truncate text-[12px] font-semibold text-(--text-strong)">{agent.name}</span>
          </div>
        )}

        <div className="nexus-memory-metrics flex min-w-0 flex-1 items-center gap-4 overflow-x-auto text-[11px] text-(--text-soft)">
          <MemoryMetric label={t("capability.memory_metric_index")} value={snapshot?.index ? t("capability.memory_ready") : "-"} />
          <MemoryMetric label={t("capability.memory_metric_topics")} value={String(counts.topics)} />
          <MemoryMetric label={t("capability.memory_metric_logs")} value={String(counts.logs)} />
          <MemoryMetric
            label={t("capability.memory_metric_updated")}
            value={latestDocument ? formatMemoryModifiedTime(latestDocument.modified_at, locale) : "-"}
          />
        </div>

        <UiIconButton
          aria-label={t("capability.refresh")}
          disabled={loading}
          onClick={() => void refresh()}
          size="md"
          title={t("capability.refresh")}
          variant="ghost"
        >
          <RefreshCw className={cn("h-4 w-4", loading && "animate-spin")} />
        </UiIconButton>
      </div>

      {loading && !snapshot ? (
        <div className="flex min-h-0 flex-1 items-center justify-center text-(--text-muted)">
          <LoaderCircle className="h-5 w-5 animate-spin" />
        </div>
      ) : error ? (
        <UiStateBlock description={error} size="sm" title={t("capability.memory_load_failed")} />
      ) : (
        <div className="nexus-memory-layout min-h-0 min-w-0 flex-1">
          <aside className="nexus-memory-catalog flex min-h-0 min-w-0 flex-col border-r border-(--divider-subtle-color) bg-(--surface-raised-background)">
            <div className="shrink-0 border-b border-(--divider-subtle-color) p-3">
              <UiSearchInput
                className="w-full"
                inputClassName="text-[12px]"
                onChange={setQuery}
                placeholder={t("capability.memory_search_placeholder")}
                value={query}
              />
              <div className="soft-scrollbar mt-2.5 flex gap-1 overflow-x-auto" role="tablist">
                {FILTERS.map((value) => (
                  <button
                    aria-selected={filter === value}
                    className={cn(
                      "shrink-0 rounded-[6px] px-2 py-1 text-[10.5px] font-medium transition-colors",
                      filter === value
                        ? "bg-(--surface-interactive-active-background) text-(--text-strong)"
                        : "text-(--text-soft) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default)",
                    )}
                    key={value}
                    onClick={() => setFilter(value)}
                    role="tab"
                    type="button"
                  >
                    {filterLabel(value)}
                  </button>
                ))}
              </div>
            </div>

            <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-2 py-2">
              {indexVisible && snapshot?.index ? (
                <div className="mb-2">
                  <MemorySectionLabel label={t("capability.memory_index")} />
                  <MemoryDocumentRow
                    document={snapshot.index}
                    isSelected={selectedPath === snapshot.index.path}
                    locale={locale}
                    onSelect={selectDocument}
                    typeLabel={t("capability.memory_index")}
                  />
                </div>
              ) : null}

              {visibleDocuments.length > 0 ? (
                <div>
                  <MemorySectionLabel label={t("capability.memory_documents")} value={String(visibleDocuments.length)} />
                  <div className="space-y-0.5">
                    {visibleDocuments.map((document) => (
                      <MemoryDocumentRow
                        document={document}
                        isSelected={selectedPath === document.path}
                        key={document.path}
                        locale={locale}
                        onSelect={selectDocument}
                        typeLabel={document.kind === "daily_log"
                          ? t("capability.memory_type_daily_log")
                          : memoryTypeLabel(document.type, t)}
                      />
                    ))}
                  </div>
                </div>
              ) : !indexVisible ? (
                <div className="px-3 py-10 text-center">
                  <Search className="mx-auto h-5 w-5 text-(--icon-muted)" />
                  <p className="mt-2 text-[12px] text-(--text-muted)">{t("capability.memory_empty_filter")}</p>
                </div>
              ) : null}

              {snapshot?.truncated ? (
                <p className="px-3 py-3 text-[10.5px] leading-4 text-(--text-soft)">{t("capability.memory_truncated")}</p>
              ) : null}
            </div>

            {snapshot?.layout === "empty" ? (
              <div className="border-t border-(--divider-subtle-color) px-4 py-4">
                <p className="text-[12px] font-semibold text-(--text-strong)">{t("capability.memory_empty_title")}</p>
                <p className="mt-1 text-[11px] leading-5 text-(--text-muted)">{t("capability.memory_empty_description")}</p>
              </div>
            ) : null}
          </aside>

          <MemoryDocumentPanel
            agentId={agent.agent_id}
            document={selectedDocument}
            onBack={() => setCompactDocumentOpen(false)}
            onSaved={() => void refresh()}
            onSelectPath={selectDocument}
          />
        </div>
      )}
    </div>
  );
}

function MemoryMetric({ label, value }: { label: string; value: string }) {
  return (
    <span className="flex shrink-0 items-baseline gap-1.5">
      <span>{label}</span>
      <strong className="font-semibold tabular-nums text-(--text-default)">{value}</strong>
    </span>
  );
}

function MemorySectionLabel({ label, value }: { label: string; value?: string }) {
  return (
    <div className="flex items-center justify-between px-2 pb-1 pt-1 text-[10px] font-semibold uppercase text-(--text-soft)">
      <span>{label}</span>
      {value ? <span className="tabular-nums">{value}</span> : null}
    </div>
  );
}

function MemoryDocumentRow({
  document,
  isSelected,
  locale,
  onSelect,
  typeLabel,
}: {
  document: MemoryDocument;
  isSelected: boolean;
  locale: string;
  onSelect: (path: string) => void;
  typeLabel: string;
}) {
  const Icon = memoryDocumentIcon(document);
  const stale = memoryAgeDays(document.modified_at) > 1;
  return (
    <button
      className={cn(
        "group relative flex w-full items-start gap-2.5 rounded-[7px] px-2.5 py-2.5 text-left transition-colors",
        isSelected
          ? "bg-(--surface-interactive-active-background)"
          : "hover:bg-(--surface-interactive-hover-background)",
      )}
      onClick={() => onSelect(document.path)}
      type="button"
    >
      {isSelected ? <span className="absolute bottom-2 left-0 top-2 w-[2px] rounded-full bg-(--primary)" /> : null}
      <span className={cn(
        "mt-0.5 flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px]",
        memoryTypeTone(document),
      )}>
        <Icon className="h-3.5 w-3.5" />
      </span>
      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="truncate text-[12px] font-semibold text-(--text-strong)">{document.title}</span>
          {document.indexed && document.kind === "topic" ? (
            <Link2 className="h-3 w-3 shrink-0 text-emerald-600 dark:text-emerald-400" />
          ) : null}
        </span>
        <span className="mt-0.5 line-clamp-2 block text-[10.5px] leading-4 text-(--text-muted)">
          {document.description || document.path}
        </span>
        <span className="mt-1 flex items-center gap-1.5 text-[9.5px] text-(--text-soft)">
          <span>{typeLabel}</span>
          <span aria-hidden="true">·</span>
          <Clock3 className="h-2.5 w-2.5" />
          <span className={stale ? "text-amber-600 dark:text-amber-400" : undefined}>
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </span>
        </span>
      </span>
    </button>
  );
}

function memoryDocumentIcon(document: MemoryDocument) {
  if (document.kind === "index") return BookOpenText;
  if (document.kind === "daily_log") return History;
  switch (document.type) {
    case "user": return UserRound;
    case "feedback": return MessageSquareWarning;
    case "project": return FolderKanban;
    case "reference": return Link2;
    default: return FileText;
  }
}

function memoryTypeTone(document: MemoryDocument): string {
  if (document.kind === "index") return "bg-sky-500/10 text-sky-600 dark:text-sky-400";
  if (document.kind === "daily_log") return "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
  switch (document.type) {
    case "user": return "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400";
    case "feedback": return "bg-amber-500/10 text-amber-700 dark:text-amber-400";
    case "project": return "bg-blue-500/10 text-blue-600 dark:text-blue-400";
    case "reference": return "bg-rose-500/10 text-rose-600 dark:text-rose-400";
    default: return "bg-zinc-500/10 text-zinc-600 dark:text-zinc-400";
  }
}

function memoryTypeLabel(type: MemoryDocumentType | undefined, t: ReturnType<typeof useI18n>["t"]): string {
  switch (type) {
    case "user": return t("capability.memory_type_user");
    case "feedback": return t("capability.memory_type_feedback");
    case "project": return t("capability.memory_type_project");
    case "reference": return t("capability.memory_type_reference");
    default: return t("capability.memory_type_topic");
  }
}

function memoryCounts(snapshot: MemorySnapshot | null): { logs: number; topics: number } {
  let logs = 0;
  let topics = 0;
  for (const document of snapshot?.documents ?? []) {
    if (document.kind === "daily_log") {
      logs++;
    } else {
      topics++;
    }
  }
  return { logs, topics };
}
