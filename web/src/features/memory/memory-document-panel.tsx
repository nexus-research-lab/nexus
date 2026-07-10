"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ArrowLeft,
  Check,
  FileText,
  Link2,
  LoaderCircle,
  Pencil,
  Save,
  X,
} from "lucide-react";

import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import {
  getWorkspaceFileContentApi,
  updateWorkspaceFileContentApi,
} from "@/lib/api/agent-manage-api";
import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { UiStateBlock } from "@/shared/ui/state-block";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type { MemoryDocument } from "@/types/memory/memory";

import {
  formatMemoryFileSize,
  formatMemoryModifiedTime,
  memoryAgeDays,
  parseMemoryIndexEntries,
  stripMemoryFrontmatter,
} from "./memory-utils";

interface MemoryDocumentPanelProps {
  agentId: string;
  document: MemoryDocument | null;
  onBack: () => void;
  onSaved: () => void;
  onSelectPath: (path: string) => void;
}

export function MemoryDocumentPanel({
  agentId,
  document,
  onBack,
  onSaved,
  onSelectPath,
}: MemoryDocumentPanelProps) {
  const { locale, t } = useI18n();
  const [content, setContent] = useState("");
  const [draft, setDraft] = useState("");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const fileStates = useWorkspaceLiveStore((state) => state.file_states);
  const liveState = document ? fileStates[`${agentId}:${document.path}`] : undefined;
  const runtimeWriting = Boolean(
    liveState && liveState.source !== "api" && liveState.status === "writing",
  );

  const loadDocument = useCallback(async () => {
    if (!document) {
      setContent("");
      setDraft("");
      return;
    }
    setLoading(true);
    setError(null);
    try {
      const response = await getWorkspaceFileContentApi(agentId, document.path);
      setContent(response.content);
      setDraft(response.content);
    } catch (loadError) {
      setError(loadError instanceof Error ? loadError.message : t("capability.memory_load_failed"));
    } finally {
      setLoading(false);
    }
  }, [agentId, document, t]);

  useEffect(() => {
    setEditing(false);
    void loadDocument();
  }, [loadDocument]);

  useEffect(() => {
    if (editing || !liveState || typeof liveState.live_content !== "string") {
      return;
    }
    setContent(liveState.live_content);
    setDraft(liveState.live_content);
  }, [editing, liveState]);

  useEffect(() => {
    if (editing || !liveState || liveState.status !== "updated" || typeof liveState.live_content === "string") {
      return;
    }
    void loadDocument();
  }, [editing, liveState, loadDocument]);

  const indexEntries = useMemo(
    () => document?.kind === "index" ? parseMemoryIndexEntries(content) : [],
    [content, document?.kind],
  );
  const staleDays = document ? memoryAgeDays(document.modified_at) : 0;
  const dirty = draft !== content;

  const save = async () => {
    if (!document || !dirty || saving || runtimeWriting) {
      return;
    }
    setSaving(true);
    setError(null);
    try {
      const response = await updateWorkspaceFileContentApi(agentId, document.path, draft);
      setContent(response.content);
      setDraft(response.content);
      setEditing(false);
      onSaved();
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : t("capability.memory_save_failed"));
    } finally {
      setSaving(false);
    }
  };

  if (!document) {
    return (
      <div className="nexus-memory-document flex min-h-0 items-center justify-center">
        <UiStateBlock
          description={t("capability.memory_select_description")}
          size="sm"
          title={t("capability.memory_select_title")}
        />
      </div>
    );
  }

  return (
    <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col bg-(--background)">
      <div className="flex min-h-[62px] shrink-0 items-center gap-3 border-b border-(--divider-subtle-color) px-4 py-2.5">
        <UiIconButton
          aria-label={t("common.back")}
          className="nexus-memory-compact-only"
          onClick={onBack}
          size="md"
          title={t("common.back")}
          variant="ghost"
        >
          <ArrowLeft className="h-4 w-4" />
        </UiIconButton>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <FileText className="h-4 w-4 shrink-0 text-(--icon-muted)" />
            <h2 className="truncate text-[14px] font-semibold text-(--text-strong)">{document.title}</h2>
            {document.indexed && document.kind === "topic" ? (
              <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-emerald-600 dark:text-emerald-400">
                <Link2 className="h-3 w-3" />
                {t("capability.memory_indexed")}
              </span>
            ) : null}
            {runtimeWriting ? (
              <span className="inline-flex shrink-0 items-center gap-1 text-[10.5px] font-medium text-(--primary)">
                <LoaderCircle className="h-3 w-3 animate-spin" />
                {t("capability.memory_runtime_writing")}
              </span>
            ) : null}
          </div>
          <div className="mt-0.5 flex min-w-0 items-center gap-2 text-[11px] text-(--text-soft)">
            <span className="truncate font-mono">{document.path}</span>
            <span aria-hidden="true">·</span>
            <span className="shrink-0">{formatMemoryFileSize(document.size)}</span>
            <span aria-hidden="true">·</span>
            <span className="shrink-0">{formatMemoryModifiedTime(document.modified_at, locale)}</span>
          </div>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
          {editing ? (
            <>
              <UiButton disabled={!dirty || saving || runtimeWriting} onClick={() => void save()} size="sm">
                {saving ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
                {t("common.save")}
              </UiButton>
              <UiIconButton
                aria-label={t("common.cancel")}
                onClick={() => {
                  setDraft(content);
                  setEditing(false);
                }}
                size="md"
                title={t("common.cancel")}
                variant="ghost"
              >
                <X className="h-4 w-4" />
              </UiIconButton>
            </>
          ) : (
            <UiIconButton
              aria-label={t("common.edit")}
              disabled={runtimeWriting}
              onClick={() => setEditing(true)}
              size="md"
              title={t("common.edit")}
              variant="ghost"
            >
              <Pencil className="h-4 w-4" />
            </UiIconButton>
          )}
        </div>
      </div>

      {staleDays > 1 ? (
        <div className="shrink-0 border-b border-amber-200/70 bg-amber-50/70 px-4 py-2 text-[11.5px] leading-5 text-amber-800 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-300">
          {t("capability.memory_stale", { count: staleDays })}
        </div>
      ) : null}

      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col overflow-y-auto">
        {loading ? (
          <div className="flex min-h-[260px] items-center justify-center text-(--text-muted)">
            <LoaderCircle className="h-5 w-5 animate-spin" />
          </div>
        ) : error ? (
          <UiStateBlock description={error} size="sm" title={t("capability.memory_load_failed")} />
        ) : editing ? (
          <textarea
            aria-label={t("capability.memory_editor_aria")}
            className="message-cjk-code-font min-h-0 w-full flex-1 resize-none overflow-y-auto bg-transparent px-5 py-4 text-[13px] leading-6 text-(--text-default) outline-none"
            onChange={(event) => setDraft(event.target.value)}
            spellCheck={false}
            value={draft}
          />
        ) : document.kind === "index" && indexEntries.length > 0 ? (
          <div className="mx-auto w-full max-w-[860px] px-5 py-5">
            <div className="mb-3 flex items-center gap-2 text-[12px] font-semibold text-(--text-muted)">
              <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              {t("capability.memory_index_entries", { count: indexEntries.length })}
            </div>
            <div className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
              {indexEntries.map((entry) => (
                <button
                  className="group flex w-full items-start gap-3 px-1 py-3.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background)"
                  key={`${entry.path}:${entry.title}`}
                  onClick={() => onSelectPath(entry.path)}
                  type="button"
                >
                  <Link2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted) group-hover:text-(--primary)" />
                  <span className="min-w-0 flex-1">
                    <span className="block text-[13px] font-semibold text-(--text-strong)">{entry.title}</span>
                    {entry.description ? (
                      <span className="mt-0.5 block text-[12px] leading-5 text-(--text-muted)">{entry.description}</span>
                    ) : null}
                    <span className="mt-1 block truncate font-mono text-[10.5px] text-(--text-soft)">{entry.path}</span>
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          <MarkdownRendererContent
            className={cn("mx-auto min-h-full w-full max-w-[860px] px-5 py-5", document.kind === "daily_log" && "font-mono")}
            content={stripMemoryFrontmatter(content)}
            mermaidShowHeader={false}
            workspaceAgentId={agentId}
          />
        )}
      </div>
    </div>
  );
}
