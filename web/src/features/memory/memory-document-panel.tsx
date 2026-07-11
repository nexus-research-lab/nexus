"use client";

import { useMemo } from "react";
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
import { useMemoryDocument } from "./use-memory-document";

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
  const liveState = useWorkspaceLiveStore((state) =>
    document ? state.file_states[`${agentId}:${document.path}`] : undefined);
  const runtimeWriting = Boolean(
    liveState && liveState.source !== "api" && liveState.status === "writing",
  );
  const controller = useMemoryDocument({
    agentId,
    document,
    fallbackLoadError: t("capability.memory_load_failed"),
    fallbackSaveError: t("capability.memory_save_failed"),
    liveState,
    onSaved,
    runtimeWriting,
  });
  const indexEntries = useMemo(
    () => document?.kind === "index"
      ? parseMemoryIndexEntries(controller.content)
      : [],
    [controller.content, document?.kind],
  );

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

  const staleDays = memoryAgeDays(document.modified_at);
  return (
    <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col bg-(--background)">
      <MemoryDocumentHeader
        controller={controller}
        document={document}
        locale={locale}
        onBack={onBack}
        runtimeWriting={runtimeWriting}
      />

      {staleDays > 1 ? (
        <div className="shrink-0 border-b border-amber-200/70 bg-amber-50/70 px-4 py-2 text-[11.5px] leading-5 text-amber-800 dark:border-amber-800/40 dark:bg-amber-950/20 dark:text-amber-300">
          {t("capability.memory_stale", { count: staleDays })}
        </div>
      ) : null}
      {controller.commandError ? (
        <div className="shrink-0 border-b border-(--divider-subtle-color) px-4 py-2 text-[11.5px] leading-5 text-(--destructive)">
          {controller.commandError}
        </div>
      ) : null}

      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col overflow-y-auto">
        {controller.isLoading ? (
          <div className="flex min-h-[260px] items-center justify-center text-(--text-muted)">
            <LoaderCircle className="h-5 w-5 animate-spin" />
          </div>
        ) : controller.resourceError ? (
          <UiStateBlock
            description={controller.resourceError}
            size="sm"
            title={t("capability.memory_load_failed")}
          />
        ) : controller.editing ? (
          <textarea
            aria-label={t("capability.memory_editor_aria")}
            className="message-cjk-code-font min-h-0 w-full flex-1 resize-none overflow-y-auto bg-transparent px-5 py-4 text-[13px] leading-6 text-(--text-default) outline-none"
            onChange={(event) => controller.setDraft(event.target.value)}
            spellCheck={false}
            value={controller.draft}
          />
        ) : document.kind === "index" && indexEntries.length > 0 ? (
          <MemoryIndexEntries
            entries={indexEntries}
            onSelectPath={onSelectPath}
          />
        ) : (
          <MarkdownRendererContent
            className={cn(
              "mx-auto min-h-full w-full max-w-[860px] px-5 py-5",
              document.kind === "daily_log" && "font-mono",
            )}
            content={stripMemoryFrontmatter(controller.content)}
            mermaidShowHeader={false}
            workspaceAgentId={agentId}
          />
        )}
      </div>
    </div>
  );
}

interface MemoryDocumentControllerView {
  cancelEditing: () => void;
  dirty: boolean;
  editing: boolean;
  isSaving: boolean;
  save: () => Promise<void>;
  startEditing: () => void;
}

function MemoryDocumentHeader({
  controller,
  document,
  locale,
  onBack,
  runtimeWriting,
}: {
  controller: MemoryDocumentControllerView;
  document: MemoryDocument;
  locale: string;
  onBack: () => void;
  runtimeWriting: boolean;
}) {
  const { t } = useI18n();
  return (
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
          <h2 className="truncate text-[14px] font-semibold text-(--text-strong)">
            {document.title}
          </h2>
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
          <span className="shrink-0">
            {formatMemoryModifiedTime(document.modified_at, locale)}
          </span>
        </div>
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {controller.editing ? (
          <>
            <UiButton
              disabled={!controller.dirty || controller.isSaving || runtimeWriting}
              onClick={() => void controller.save()}
              size="sm"
            >
              {controller.isSaving
                ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                : <Save className="h-3.5 w-3.5" />}
              {t("common.save")}
            </UiButton>
            <UiIconButton
              aria-label={t("common.cancel")}
              disabled={controller.isSaving}
              onClick={controller.cancelEditing}
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
            onClick={controller.startEditing}
            size="md"
            title={t("common.edit")}
            variant="ghost"
          >
            <Pencil className="h-4 w-4" />
          </UiIconButton>
        )}
      </div>
    </div>
  );
}

function MemoryIndexEntries({
  entries,
  onSelectPath,
}: {
  entries: ReturnType<typeof parseMemoryIndexEntries>;
  onSelectPath: (path: string) => void;
}) {
  const { t } = useI18n();
  return (
    <div className="mx-auto w-full max-w-[860px] px-5 py-5">
      <div className="mb-3 flex items-center gap-2 text-[12px] font-semibold text-(--text-muted)">
        <Check className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
        {t("capability.memory_index_entries", { count: entries.length })}
      </div>
      <div className="divide-y divide-(--divider-subtle-color) border-y border-(--divider-subtle-color)">
        {entries.map((entry) => (
          <button
            className="group flex w-full items-start gap-3 px-1 py-3.5 text-left transition-colors hover:bg-(--surface-interactive-hover-background)"
            key={`${entry.path}:${entry.title}`}
            onClick={() => onSelectPath(entry.path)}
            type="button"
          >
            <Link2 className="mt-0.5 h-3.5 w-3.5 shrink-0 text-(--icon-muted) group-hover:text-(--primary)" />
            <span className="min-w-0 flex-1">
              <span className="block text-[13px] font-semibold text-(--text-strong)">
                {entry.title}
              </span>
              {entry.description ? (
                <span className="mt-0.5 block text-[12px] leading-5 text-(--text-muted)">
                  {entry.description}
                </span>
              ) : null}
              <span className="mt-1 block truncate font-mono text-[10.5px] text-(--text-soft)">
                {entry.path}
              </span>
            </span>
          </button>
        ))}
      </div>
    </div>
  );
}
