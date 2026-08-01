"use client";

import { useMemo } from "react";
import { LoaderCircle } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { useWorkspaceLiveStore } from "@/store/workspace-live";
import type { WorkspaceLiveFileState } from "@/types/app/workspace-live";
import type { MemoryDocument } from "@/types/memory/memory";

import {
  MEMORY_STALE_AFTER_DAYS,
  memoryAgeDays,
  stripMemoryFrontmatter,
} from "../memory-utils";
import { MemoryIndexEntries } from "./index/memory-index-entries";
import { parseMemoryIndexEntries } from "./index/memory-index-model";
import { MemoryDocumentHeader } from "./memory-document-header";
import { useMemoryDocument } from "./use-memory-document";

interface MemoryDocumentPanelProps {
  agentId: string;
  document: MemoryDocument | null;
  onBack: () => void;
  onSaved: () => void;
  onSelectPath: (path: string) => void;
}

type MemoryDocumentController = ReturnType<typeof useMemoryDocument>;

export function MemoryDocumentPanel({
  agentId,
  document,
  onBack,
  onSaved,
  onSelectPath,
}: MemoryDocumentPanelProps) {
  const { locale, t } = useI18n();
  const liveState = useMemoryLiveFileState(agentId, document);
  const runtimeWriting = isRuntimeWriting(liveState);
  const controller = useMemoryDocument({
    agentId,
    document,
    fallbackLoadError: t("capability.memory_load_failed"),
    fallbackSaveError: t("capability.memory_save_failed"),
    liveState,
    onSaved,
    runtimeWriting,
  });

  if (!document) {
    return <MemoryDocumentEmpty />;
  }
  return (
    <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col">
      <MemoryDocumentHeader
        controller={controller}
        document={document}
        locale={locale}
        onBack={onBack}
        runtimeWriting={runtimeWriting}
      />
      <MemoryDocumentAlerts controller={controller} document={document} />
      <div className="soft-scrollbar flex min-h-0 flex-1 flex-col overflow-y-auto">
        <MemoryDocumentBody
          agentId={agentId}
          controller={controller}
          document={document}
          onSelectPath={onSelectPath}
        />
      </div>
    </div>
  );
}

function useMemoryLiveFileState(
  agentId: string,
  document: MemoryDocument | null,
): WorkspaceLiveFileState | undefined {
  const scopeKey = document ? `${agentId}:${document.path}` : null;
  return useWorkspaceLiveStore((state) => (
    scopeKey ? state.file_states[scopeKey] : undefined
  ));
}

function isRuntimeWriting(liveState?: WorkspaceLiveFileState): boolean {
  return liveState?.source !== "api" && liveState?.status === "writing";
}

function MemoryDocumentEmpty() {
  const { t } = useI18n();
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

function MemoryDocumentAlerts({
  controller,
  document,
}: {
  controller: MemoryDocumentController;
  document: MemoryDocument;
}) {
  const { t } = useI18n();
  const staleDays = memoryAgeDays(document.modified_at);
  const stale = staleDays > MEMORY_STALE_AFTER_DAYS;
  if (!stale && !controller.commandError) {
    return null;
  }
  return (
    <div className="nexus-memory-document-content shrink-0 space-y-1 pb-2">
      {stale ? (
        <div className="rounded-[8px] bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)] px-3 py-2 text-compact leading-5 text-(--warning)">
          {t("capability.memory_stale", { count: staleDays })}
        </div>
      ) : null}
      {controller.commandError ? (
        <div className="rounded-[8px] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-3 py-2 text-compact leading-5 text-(--destructive)">
          {controller.commandError}
        </div>
      ) : null}
    </div>
  );
}

function MemoryDocumentBody({
  agentId,
  controller,
  document,
  onSelectPath,
}: {
  agentId: string;
  controller: MemoryDocumentController;
  document: MemoryDocument;
  onSelectPath: (path: string) => void;
}) {
  const { t } = useI18n();
  const indexEntries = useMemo(
    () => document.kind === "index"
      ? parseMemoryIndexEntries(controller.content)
      : [],
    [controller.content, document.kind],
  );
  if (controller.isLoading) {
    return (
      <div className="flex min-h-[260px] items-center justify-center text-(--text-muted)">
        <LoaderCircle className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (controller.resourceError) {
    return (
      <UiStateBlock
        description={controller.resourceError}
        size="sm"
        title={t("capability.memory_load_failed")}
      />
    );
  }
  if (controller.editing) {
    return (
      <textarea
        aria-label={t("capability.memory_editor_aria")}
        className="nexus-memory-document-content message-cjk-code-font min-h-0 flex-1 resize-none overflow-y-auto bg-transparent py-4 text-sm leading-6 text-(--text-default) outline-none"
        onChange={(event) => controller.setDraft(event.target.value)}
        spellCheck={false}
        value={controller.draft}
      />
    );
  }
  if (document.kind === "index" && indexEntries.length > 0) {
    return (
      <MemoryIndexEntries
        entries={indexEntries}
        onSelectPath={onSelectPath}
      />
    );
  }
  return (
    <UiMarkdownContent
      className={cn(
        "nexus-memory-document-content min-h-full py-5",
        document.kind === "daily_log" && "font-mono",
      )}
      content={stripMemoryFrontmatter(controller.content)}
      mermaidShowHeader={false}
      workspaceAgentId={agentId}
    />
  );
}
