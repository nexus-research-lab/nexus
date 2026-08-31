/**
 * INPUT: exact WorkGraph preview、源 Session 与源会话可见 Agent。
 * OUTPUT: 补载全局 Agent 目录解析隐藏编辑 Agent，展示专用 DM、画布、版本选择和区分读取/写入结果的恢复状态。
 * POS: 关闭页面不删除会话；应用只投影所选版本，unknown 写入在当前页面锁定且不改写源 Execution/聊天。
 */
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, History, LoaderCircle, RefreshCw } from "lucide-react";

import { DmChatPanel } from "@/features/conversation/room/dm/panel/dm-chat-panel";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import {
  applyWorkGraphWorkflowEditorApi,
  getWorkGraphWorkflowEditorApi,
  selectWorkGraphWorkflowEditorVersionApi,
  startWorkGraphWorkflowEditorApi,
} from "@/lib/api/conversation/execution-api";
import {
  getErrorMessage,
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { useAgentStore } from "@/store/agent";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogCloseButton,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { Agent } from "@/types/agent/agent";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { ExecutionResource } from "./use-execution-resource";
import type {
  WorkGraphWorkflowEditorSession,
  WorkGraphWorkflowPreview,
} from "@/types/conversation/workgraph-workflow";

import { ExecutionWorkGraphCanvas } from "./execution-workgraph-canvas";
import { projectWorkGraphWorkflowCanvasExecution } from "./workgraph-workflow-canvas-model";

interface WorkGraphMetadataEditorDialogProps {
  agents: readonly Agent[];
  onApply: (preview: WorkGraphWorkflowPreview) => void | Promise<void>;
  onClose: () => void;
  preview: WorkGraphWorkflowPreview;
  sessionKey: string;
}

type WorkGraphEditorFailure =
  | {
      kind: "refresh";
      message: string;
    }
  | {
      kind: "start";
      message: string;
    }
  | {
      effect: MutationFailureEffect;
      kind: "apply" | "select";
      message: string;
      reconcileMessage?: string;
      selectedRevision?: number;
    }
  | {
      appliedPreview: WorkGraphWorkflowPreview;
      kind: "apply_projection";
      message: string;
    };

const EMPTY_EXECUTION_RESOURCE: ExecutionResource = {
  dismiss: () => undefined,
  error: null,
  execution: null,
  isLoading: false,
  isStale: false,
  lastSuccessfulAt: null,
  refresh: () => undefined,
  sessionKey: null,
};

export function WorkGraphMetadataEditorDialog({
  agents,
  onApply,
  onClose,
  preview,
  sessionKey,
}: WorkGraphMetadataEditorDialogProps) {
  const { locale, t } = useI18n();
  const runtimeKind = useDefaultAgentRuntimeKind();
  const initialPreviewRef = useRef(preview);
  const sourceAgentsRef = useRef(agents);
  const catalogAgents = useAgentStore((state) => state.agents);
  const loadAgents = useAgentStore((state) => state.load_agents_from_server);
  const startContextRef = useRef({
    locale,
    failureMessage: t("execution.workflow_editor_start_failed"),
  });
  const editorRef = useRef<WorkGraphWorkflowEditorSession | null>(null);
  const [editor, setEditor] = useState<WorkGraphWorkflowEditorSession | null>(null);
  const [busy, setBusy] = useState(false);
  const [applying, setApplying] = useState(false);
  const [loading, setLoading] = useState(true);
  const [selectingRevision, setSelectingRevision] = useState<number | null>(null);
  const [failure, setFailure] = useState<WorkGraphEditorFailure | null>(null);
  const [startAttempt, setStartAttempt] = useState(0);

  const updateEditor = useCallback((next: WorkGraphWorkflowEditorSession) => {
    editorRef.current = next;
    setEditor(next);
  }, []);

  useEffect(() => {
    let active = true;
    const initialPreview = initialPreviewRef.current;
    const startContext = startContextRef.current;
    void startWorkGraphWorkflowEditorApi(sessionKey, initialPreview, startContext.locale)
      .then(async (session) => {
        if (!active) return;
        const hasEditorAgent = sourceAgentsRef.current.some(
          (item) => item.agent_id === session.agent_id,
        ) || useAgentStore.getState().get_agent(session.agent_id) !== undefined;
        if (!hasEditorAgent) {
          await loadAgents();
        }
        if (!active) return;
        updateEditor(session);
      })
      .catch((reason: unknown) => {
        if (active) {
          setFailure({
            kind: "start",
            message: getErrorMessage(reason, startContext.failureMessage),
          });
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, [loadAgents, sessionKey, startAttempt, updateEditor]);

  const handleRetryStart = useCallback(() => {
    setFailure(null);
    setLoading(true);
    setStartAttempt((attempt) => attempt + 1);
  }, []);

  const refreshEditor = useCallback(async () => {
    const current = editorRef.current;
    if (!current) return current;
    try {
      const next = await getWorkGraphWorkflowEditorApi(sessionKey, current.editor_id);
      updateEditor(next);
      setFailure((existing) => {
        if (!existing || existing.kind === "refresh") {
          return null;
        }
        if (
          existing.kind === "select"
          && existing.selectedRevision === next.selected_revision
        ) {
          return null;
        }
        return existing;
      });
      return next;
    } catch (reason: unknown) {
      const message = getErrorMessage(
        reason,
        t("execution.workflow_editor_refresh_failed"),
      );
      setFailure((existing) => {
        if (existing?.kind === "apply" || existing?.kind === "select") {
          return { ...existing, reconcileMessage: message };
        }
        if (existing?.kind === "apply_projection") {
          return existing;
        }
        return { kind: "refresh", message };
      });
      return null;
    }
  }, [sessionKey, t, updateEditor]);

  const handleSnapshotChange = useCallback((_snapshot: SessionSnapshotPayload) => {
    void refreshEditor();
  }, [refreshEditor]);

  const handleClose = useCallback(() => onClose(), [onClose]);

  const handleSelectRevision = useCallback(async (selectedRevision: number) => {
    const current = editorRef.current;
    if (!current || busy || applying || selectingRevision !== null || current.selected_revision === selectedRevision) return;
    setSelectingRevision(selectedRevision);
    setFailure(null);
    try {
      const next = await selectWorkGraphWorkflowEditorVersionApi(
        sessionKey,
        current.editor_id,
        current.revision,
        selectedRevision,
      );
      updateEditor(next);
    } catch (reason: unknown) {
      const mutation = projectMutationFailure(
        reason,
        t("execution.workflow_editor_version_failed"),
      );
      setFailure({
        effect: mutation.effect,
        kind: "select",
        message: mutation.message,
        selectedRevision,
      });
    } finally {
      setSelectingRevision(null);
    }
  }, [applying, busy, selectingRevision, sessionKey, t, updateEditor]);

  const handleApply = useCallback(async () => {
    if (failure && (
      failure.kind === "apply_projection" || (
        (failure.kind === "apply" || failure.kind === "select")
        && failure.effect !== "not_applied"
      )
    )) return;
    const current = await refreshEditor();
    if (!current || busy || applying) return;
    setApplying(true);
    setFailure(null);
    try {
      const applied = await applyWorkGraphWorkflowEditorApi(
        sessionKey,
        current.editor_id,
        current.revision,
      );
      try {
        await onApply(applied);
      } catch (reason: unknown) {
        setFailure({
          appliedPreview: applied,
          kind: "apply_projection",
          message: getErrorMessage(
            reason,
            t("execution.workflow_editor_projection_failed"),
          ),
        });
      }
    } catch (reason: unknown) {
      const mutation = projectMutationFailure(
        reason,
        t("execution.workflow_editor_apply_failed"),
      );
      setFailure({
        effect: mutation.effect,
        kind: "apply",
        message: mutation.message,
      });
    } finally {
      setApplying(false);
    }
  }, [applying, busy, failure, onApply, refreshEditor, sessionKey, t]);

  const handleRetryProjection = useCallback(async () => {
    if (!failure || failure.kind !== "apply_projection" || applying) return;
    setApplying(true);
    try {
      await onApply(failure.appliedPreview);
      setFailure(null);
    } catch (reason: unknown) {
      setFailure({
        ...failure,
        message: getErrorMessage(
          reason,
          t("execution.workflow_editor_projection_failed"),
        ),
      });
    } finally {
      setApplying(false);
    }
  }, [applying, failure, onApply, t]);

  const sessionIdentity = useMemo(() => editor ? {
    agent_id: editor.agent_id,
    chat_type: "dm" as const,
    session_key: editor.session_key,
  } : null, [editor]);
  const agent = useMemo(
    () => editor
      ? agents.find((item) => item.agent_id === editor.agent_id)
        ?? catalogAgents.find((item) => item.agent_id === editor.agent_id)
        ?? null
      : null,
    [agents, catalogAgents, editor],
  );
  const currentPreview = editor?.preview ?? preview;
  const mutationBlocked = failure?.kind === "apply_projection" || (
    (failure?.kind === "apply" || failure?.kind === "select")
    && failure.effect !== "not_applied"
  );
  const canvasExecution = useMemo(
    () => projectWorkGraphWorkflowCanvasExecution(
      currentPreview,
      editor?.revision ?? 1,
    ),
    [currentPreview, editor?.revision],
  );

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[10000]"
        labelledBy="workgraph-metadata-editor-title"
        onClose={handleClose}
      >
        <UiDialogShell
          className="pointer-events-auto relative h-[min(840px,calc(100dvh-56px))] max-h-[calc(100dvh-56px)]"
          size="wide"
          style={{ maxWidth: "min(1440px, calc(100vw - 56px))" }}
        >
          <h2 className="sr-only" id="workgraph-metadata-editor-title">
            {currentPreview.title}
          </h2>
          <UiDialogCloseButton
            className="absolute right-5 top-5 z-30 bg-(--surface-control-background) shadow-(--surface-control-shadow)"
            onClose={handleClose}
          />
          <UiDialogBody className="grid min-h-0 flex-1 overflow-hidden p-0 md:grid-cols-[minmax(360px,0.42fr)_minmax(0,0.58fr)]">
            <div className="relative flex min-h-0 min-w-0 flex-col overflow-hidden border-r border-(--divider-subtle-color) bg-(--surface-muted-background) [--conversation-composer-backdrop:var(--surface-muted-background)]">
              {loading ? (
                <div className="grid min-h-0 flex-1 place-items-center text-xs text-(--text-muted)">
                  <span className="inline-flex items-center gap-2">
                    <LoaderCircle className="h-4 w-4 animate-spin" />
                    {t("execution.workflow_editor_starting")}
                  </span>
                </div>
              ) : agent && editor ? (
                <DmChatPanel
                  currentAgent={agent}
                  embeddedEditor={{
                    introduction: {
                      description: t("execution.workflow_editor_intro_description"),
                      examples: [],
                      examplesLabel: "",
                      footer: "",
                      title: t("execution.workflow_editor_intro_title"),
                    },
                    placeholder: t("execution.workflow_editor_placeholder"),
                    visibleAfterUnixMilli: editor.display_after_unix_milli,
                  }}
                  executionResource={EMPTY_EXECUTION_RESOURCE}
                  layout="desktop"
                  runtimeKind={runtimeKind}
                  sessionIdentity={sessionIdentity}
                  todos={[]}
                  onBusyChange={setBusy}
                  onConversationSnapshotChange={handleSnapshotChange}
                />
              ) : (
                <div className="grid min-h-0 flex-1 place-items-center px-5 text-center">
                  <div className="flex max-w-72 flex-col items-center gap-3">
                    <UiResourceState
                      className="min-h-0 py-4"
                      description={failure?.message ?? t("execution.workflow_editor_start_failed")}
                      impact={t("execution.workflow_editor_start_failed_impact")}
                      nextStep={t("execution.workflow_editor_start_failed_next_step")}
                      primaryAction={{
                        label: t("execution.workflow_editor_retry"),
                        onClick: handleRetryStart,
                      }}
                      size="sm"
                      state="error"
                      title={t("execution.workflow_editor_start_failed")}
                      urgency="polite"
                      variant="plain"
                    />
                  </div>
                </div>
              )}
            </div>

            <div className="flex min-h-0 min-w-0 flex-col overflow-hidden bg-(--surface-canvas-background)">
              <div className="shrink-0 border-b border-(--divider-subtle-color) px-8 py-5 pr-16">
                <div className="flex min-w-0 items-start justify-between gap-6">
                  <div className="min-w-0">
                    <h3 className="truncate text-[19px] font-semibold leading-7 tracking-[-0.015em] text-(--text-strong)">{currentPreview.title}</h3>
                    <code className="mt-1 block text-xs text-(--text-soft)">/{currentPreview.slash_name}</code>
                  </div>
                  <button
                    className={getDialogActionClassName("primary", "compact")}
                    disabled={!editor || busy || applying || mutationBlocked}
                    type="button"
                    onClick={() => void handleApply()}
                  >
                    {applying ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                    {t("execution.workflow_editor_apply")}
                  </button>
                </div>
                {editor && editor.versions.length > 1 ? (
                  <div className="mt-4 flex min-w-0 items-center gap-2 border-t border-(--divider-subtle-color) pt-3">
                    <span className="inline-flex shrink-0 items-center gap-1.5 text-[11px] font-medium text-(--text-muted)">
                      <History className="h-3.5 w-3.5" />
                      {t("execution.workflow_editor_versions")}
                    </span>
                    <div className="flex min-w-0 flex-1 gap-1.5 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                      {editor.versions.map((version) => (
                        <button
                          key={version.revision}
                          aria-pressed={version.selected}
                          className={`inline-flex shrink-0 items-center gap-1 rounded-full border px-2.5 py-1 text-[11px] transition-colors ${
                            version.selected
                              ? "border-[color:color-mix(in_srgb,var(--primary)_36%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] font-semibold text-(--primary)"
                              : "border-(--divider-subtle-color) bg-(--surface-control-background) text-(--text-muted) hover:text-(--text-strong)"
                          }`}
                          disabled={busy || applying || selectingRevision !== null || mutationBlocked}
                          title={`${version.title} · ${version.node_count} ${t("execution.workflow_editor_version_nodes")}`}
                          type="button"
                          onClick={() => void handleSelectRevision(version.revision)}
                        >
                          {version.selected ? <Check className="h-3 w-3" /> : null}
                          v{version.revision}
                          {selectingRevision === version.revision ? <LoaderCircle className="h-3 w-3 animate-spin" /> : null}
                        </button>
                      ))}
                    </div>
                  </div>
                ) : null}
                {failure && editor ? (
                  <WorkGraphEditorFailureState
                    applying={applying}
                    failure={failure}
                    onRefresh={() => void refreshEditor()}
                    onRetryProjection={() => void handleRetryProjection()}
                  />
                ) : null}
              </div>
              <div className="flex min-h-0 flex-1">
                <ExecutionWorkGraphCanvas
                  key={editor?.revision ?? 0}
                  currentId={null}
                  directory={{}}
                  execution={canvasExecution}
                  taskRuns={[]}
                />
              </div>
            </div>
          </UiDialogBody>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function WorkGraphEditorFailureState({
  applying,
  failure,
  onRefresh,
  onRetryProjection,
}: {
  applying: boolean;
  failure: WorkGraphEditorFailure;
  onRefresh: () => void;
  onRetryProjection: () => void;
}) {
  const { t } = useI18n();
  const description = failure.kind === "apply" || failure.kind === "select"
    ? [failure.message, failure.reconcileMessage].filter(Boolean).join(" ")
    : failure.message;
  if (failure.kind === "refresh") {
    return (
      <UiResourceState
        className="mt-3 min-h-0 py-3"
        description={description}
        impact={t("execution.workflow_editor_refresh_failed_impact")}
        nextStep={t("execution.workflow_editor_refresh_failed_next_step")}
        primaryAction={{
          icon: <RefreshCw className="h-3.5 w-3.5" />,
          label: t("execution.workflow_editor_refresh_action"),
          onClick: onRefresh,
        }}
        size="sm"
        state="error"
        title={t("execution.workflow_editor_refresh_failed")}
        urgency="polite"
        variant="card"
      />
    );
  }
  if (failure.kind === "apply_projection") {
    return (
      <UiResourceState
        className="mt-3 min-h-0 py-3"
        description={description}
        impact={t("execution.workflow_editor_projection_failed_impact")}
        nextStep={t("execution.workflow_editor_projection_failed_next_step")}
        primaryAction={{
          busy: applying,
          label: t("execution.workflow_editor_projection_retry"),
          onClick: onRetryProjection,
        }}
        size="sm"
        state="error"
        title={t("execution.workflow_editor_projection_failed_title")}
        urgency="polite"
        variant="card"
      />
    );
  }
  if (failure.kind === "start") return null;

  const notApplied = failure.effect === "not_applied";
  const selecting = failure.kind === "select";
  const impactKey = selecting
    ? notApplied
      ? "execution.workflow_editor_select_not_applied_impact"
      : "execution.workflow_editor_select_unknown_impact"
    : notApplied
      ? "execution.workflow_editor_apply_not_applied_impact"
      : "execution.workflow_editor_apply_unknown_impact";
  const nextStepKey = selecting
    ? notApplied
      ? "execution.workflow_editor_select_not_applied_next_step"
      : "execution.workflow_editor_select_unknown_next_step"
    : notApplied
      ? "execution.workflow_editor_apply_not_applied_next_step"
      : "execution.workflow_editor_apply_unknown_next_step";
  const titleKey = selecting
    ? notApplied
      ? "execution.workflow_editor_select_not_applied_title"
      : "execution.workflow_editor_select_unknown_title"
    : notApplied
      ? "execution.workflow_editor_apply_not_applied_title"
      : "execution.workflow_editor_apply_unknown_title";
  return (
    <UiResourceState
      className="mt-3 min-h-0 py-3"
      description={description}
      impact={t(impactKey)}
      nextStep={t(nextStepKey)}
      primaryAction={!notApplied && selecting ? {
        icon: <RefreshCw className="h-3.5 w-3.5" />,
        label: t("execution.workflow_editor_refresh_action"),
        onClick: onRefresh,
      } : undefined}
      size="sm"
      state="error"
      title={t(titleKey)}
      urgency="polite"
      variant="card"
    />
  );
}
