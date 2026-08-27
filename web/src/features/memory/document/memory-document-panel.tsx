"use client";

import { useMemo } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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
  deleteBusy: boolean;
  deleteError: string | null;
  deleting: boolean;
  document: MemoryDocument | null;
  onBack: () => void;
  onDelete: () => void;
  onSaved: () => void;
  onSelectPath: (path: string) => void;
}

type MemoryDocumentController = ReturnType<typeof useMemoryDocument>;

export function MemoryDocumentPanel({
  agentId,
  deleteBusy,
  deleteError,
  deleting,
  document,
  onBack,
  onDelete,
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
  if (controller.resourceError?.access) {
    return (
      <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col">
        <UiResourceState
          description={controller.resourceError.message}
          impact={t("state.access_failure_impact")}
          nextStep={t("state.permission_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: () => void controller.reload(),
          }}
          secondaryAction={{
            label: t("common.back"),
            onClick: onBack,
          }}
          size="sm"
          state="error"
          title={t("state.permission_title")}
        />
      </div>
    );
  }
  return (
    <div className="nexus-memory-document flex min-h-0 min-w-0 flex-col">
      <MemoryDocumentHeader
        controller={controller}
        deleteBusy={deleteBusy}
        deleting={deleting}
        document={document}
        locale={locale}
        onBack={onBack}
        onDelete={onDelete}
        runtimeWriting={runtimeWriting}
      />
      <MemoryDocumentAlerts
        controller={controller}
        document={document}
        externalError={deleteError}
      />
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
  externalError,
}: {
  controller: MemoryDocumentController;
  document: MemoryDocument;
  externalError: string | null;
}) {
  const { t } = useI18n();
  const staleDays = memoryAgeDays(document.modified_at);
  const stale = staleDays > MEMORY_STALE_AFTER_DAYS;
  const commandError = controller.commandError || externalError;
  const resourceFailure = controller.resourceError;
  if (
    !stale
    && !commandError
    && !(resourceFailure && !resourceFailure.access && controller.content)
  ) {
    return null;
  }
  return (
    <div className="nexus-memory-document-content shrink-0 space-y-1 pb-2">
      {stale ? (
        <div className="rounded-[8px] bg-[color:color-mix(in_srgb,var(--warning)_7%,transparent)] px-3 py-2 text-compact leading-5 text-(--warning)">
          {t("capability.memory_stale", { count: staleDays })}
        </div>
      ) : null}
      {commandError ? (
        <div className="rounded-[8px] bg-[color:color-mix(in_srgb,var(--destructive)_7%,transparent)] px-3 py-2 text-compact leading-5 text-(--destructive)">
          {commandError}
        </div>
      ) : null}
      {resourceFailure && !resourceFailure.access && controller.content ? (
        <UiResourceState
          className="min-h-0 py-3"
          description={resourceFailure.message}
          impact={t("capability.memory_stale_document_impact")}
          nextStep={t("state.retry_next_step")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: () => void controller.reload(),
          }}
          role="status"
          size="sm"
          state="error"
          title={t("capability.memory_document_refresh_failed")}
        />
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
  if (controller.isLoading && !controller.content) {
    return (
      <div className="flex min-h-[260px] items-center justify-center text-(--text-muted)">
        <LoaderCircle className="h-5 w-5 animate-spin" />
      </div>
    );
  }
  if (controller.resourceError && !controller.content) {
    return (
      <UiResourceState
        description={controller.resourceError.message}
        impact={t("state.read_failure_impact")}
        nextStep={t("state.retry_next_step")}
        primaryAction={{
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("state.retry"),
          onClick: () => void controller.reload(),
        }}
        size="sm"
        state="error"
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
