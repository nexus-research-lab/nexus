// INPUT: Memory 文档控制面、目录动作和 workspace live 状态。
// OUTPUT: 正文、P/I/R 失败、冲突双版对照与明确决策入口。
// POS: Memory 正文可视化；不自动合并或覆盖并发版本。
"use client";

import { useMemo } from "react";
import { LoaderCircle, RefreshCw } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiInlineNotice } from "@/shared/ui/feedback/inline-notice";
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
          impact={t("state.access_failure_impact")}
          primaryAction={{
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: () => void controller.reload(),
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
}: {
  controller: MemoryDocumentController;
  document: MemoryDocument;
}) {
  const { t } = useI18n();
  const staleDays = memoryAgeDays(document.modified_at);
  const stale = staleDays > MEMORY_STALE_AFTER_DAYS;
  const commandError = controller.commandError;
  const resourceFailure = controller.resourceError;
  if (
    !stale
    && !commandError
    && !controller.saveIssue
    && !(resourceFailure && !resourceFailure.access && controller.content)
  ) {
    return null;
  }
  return (
    <div className="nexus-memory-document-content shrink-0 space-y-1 pb-2">
      {stale ? (
        <UiInlineNotice
          message={t("capability.memory_stale", { count: staleDays })}
          tone="warning"
        />
      ) : null}
      {commandError ? (
        <UiResourceState
          className="min-h-0 py-3"
          impact={t("feedback.unconfirmed_impact")}
          primaryAction={{
            busy: controller.isLoading,
            icon: <RefreshCw className="h-3.5 w-3.5" />,
            label: t("capability.memory_check_save_result"),
            onClick: () => void controller.reload(),
          }}
          size="sm"
          state="error"
          title={t("capability.memory_save_failed")}
        />
      ) : null}
      {controller.saveIssue ? (
        <MemorySaveIssueNotice controller={controller} />
      ) : null}
      {resourceFailure && !resourceFailure.access && controller.content ? (
        <UiResourceState
          className="min-h-0 py-3"
          impact={t("capability.memory_stale_document_impact")}
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

function MemorySaveIssueNotice({
  controller,
}: {
  controller: MemoryDocumentController;
}) {
  const { t } = useI18n();
  const issue = controller.saveIssue;
  if (!issue) {
    return null;
  }
  if (issue.kind === "conflict") {
    const reviewing = issue.phase === "review";
    if (reviewing) {
      return (
        <UiResourceState
          className="min-h-0 py-3"
          impact={t("capability.memory_conflict_review_impact")}
          primaryAction={{
            label: t("capability.memory_use_latest"),
            onClick: controller.adoptLatest,
          }}
          secondaryAction={{
            busy: controller.isSaving,
            disabled: !controller.revision,
            label: t("capability.memory_overwrite_draft"),
            onClick: () => void controller.overwriteConflict(),
            tone: "danger",
          }}
          size="sm"
          state="decision"
          title={t("capability.memory_conflict_review_title")}
          tone="warning"
        />
      );
    }
    return (
      <UiResourceState
        className="min-h-0 py-3"
        impact={t("capability.memory_conflict_impact")}
        primaryAction={{
          busy: controller.isLoading,
          busyLabel: t("capability.memory_loading_latest"),
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("capability.memory_load_latest"),
          onClick: () => void controller.reload(),
        }}
        size="sm"
        state="error"
        title={t("capability.memory_conflict_title")}
      />
    );
  }
  if (issue.kind === "outcome_unknown") {
    return (
      <UiResourceState
        className="min-h-0 py-3"
        impact={t("capability.memory_save_unknown_impact")}
        primaryAction={{
          busy: controller.isReconciling,
          busyLabel: t("capability.memory_checking_save_result"),
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("capability.memory_check_save_result"),
          onClick: () => void controller.reconcileSave(),
        }}
        size="sm"
        state="error"
        title={t(issue.reconciliationFailed
          ? "capability.memory_save_check_failed_title"
          : "capability.memory_save_unknown_title")}
      />
    );
  }
  return (
    <UiResourceState
      className="min-h-0 py-3"
      impact={t("capability.memory_not_applied_impact")}
      nextStep={t("capability.memory_not_applied_next_step")}
      size="sm"
      state="error"
      title={t("capability.memory_not_applied_title")}
    />
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
        impact={t("state.read_failure_impact")}
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
    if (
      controller.saveIssue?.kind === "conflict"
      && controller.saveIssue.phase === "review"
    ) {
      return <MemoryConflictReview controller={controller} />;
    }
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

function MemoryConflictReview({
  controller,
}: {
  controller: MemoryDocumentController;
}) {
  const { t } = useI18n();
  return (
    <div className="nexus-memory-document-content grid min-h-0 flex-1 gap-2 py-4 lg:grid-cols-2">
      <section className="flex min-h-[240px] min-w-0 flex-col rounded-[10px] border border-[color:color-mix(in_srgb,var(--warning)_26%,var(--border-subtle))] bg-[color:color-mix(in_srgb,var(--warning)_4%,transparent)]">
        <h3 className="shrink-0 px-3 pb-2 pt-3 text-xs font-semibold text-(--text-strong)">
          {t("capability.memory_local_draft")}
        </h3>
        <textarea
          aria-label={t("capability.memory_local_draft")}
          className="message-cjk-code-font min-h-[200px] flex-1 resize-none overflow-auto bg-transparent px-3 pb-3 text-sm leading-6 text-(--text-default) outline-none"
          onChange={(event) => controller.setDraft(event.target.value)}
          spellCheck={false}
          value={controller.draft}
        />
      </section>
      <section className="flex min-h-[240px] min-w-0 flex-col rounded-[10px] border border-(--border-subtle) bg-(--surface-subtle)">
        <h3 className="shrink-0 px-3 pb-2 pt-3 text-xs font-semibold text-(--text-strong)">
          {t("capability.memory_saved_version")}
        </h3>
        <textarea
          aria-label={t("capability.memory_saved_version")}
          className="message-cjk-code-font min-h-[200px] flex-1 resize-none overflow-auto bg-transparent px-3 pb-3 text-sm leading-6 text-(--text-muted) outline-none"
          readOnly
          spellCheck={false}
          value={controller.content}
        />
      </section>
    </div>
  );
}
