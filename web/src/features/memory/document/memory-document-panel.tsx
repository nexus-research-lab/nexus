"use client";

import { useMemo } from "react";
import { LoaderCircle } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/state-block";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type { MemoryDocument } from "@/types/memory/memory";

import {
  memoryAgeDays,
  parseMemoryIndexEntries,
  stripMemoryFrontmatter,
} from "../memory-utils";
import { MemoryDocumentHeader } from "./memory-document-header";
import { MemoryIndexEntries } from "./memory-index-entries";
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
          <UiMarkdownContent
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
