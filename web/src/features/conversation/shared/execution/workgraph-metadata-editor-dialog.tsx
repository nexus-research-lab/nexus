/**
 * INPUT: exact WorkGraph preview 与源 Session。
 * OUTPUT: 左侧标准 DM 对话 + 右侧实时草图预览的短期 fork 编辑页。
 * POS: 只装配既有草图与 DM 组件的对话编辑页；目录刷新不重建临时 Session，应用前不改写原 preview。
 */
"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { LoaderCircle, MessageSquareText } from "lucide-react";

import { DmChatPanel } from "@/features/conversation/room/dm/panel/dm-chat-panel";
import { useDefaultAgentRuntimeKind } from "@/hooks/settings/use-default-agent-runtime-kind";
import {
  applyWorkGraphWorkflowEditorApi,
  closeWorkGraphWorkflowEditorApi,
  getWorkGraphWorkflowEditorApi,
  startWorkGraphWorkflowEditorApi,
} from "@/lib/api/conversation/execution-api";
import { getErrorMessage } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogBody,
  UiDialogFooter,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { getDialogActionClassName } from "@/shared/ui/dialog/dialog-styles";
import type { Agent } from "@/types/agent/agent";
import type { SessionSnapshotPayload } from "@/types/conversation/conversation";
import type { ExecutionResource } from "./use-execution-resource";
import type {
  WorkGraphWorkflowEditorSession,
  WorkGraphWorkflowPreview,
} from "@/types/conversation/workgraph-workflow";

import { NamedWorkGraphSketch } from "./named-workgraph-sketch";

interface WorkGraphMetadataEditorDialogProps {
  agents: readonly Agent[];
  onApply: (preview: WorkGraphWorkflowPreview) => void;
  onClose: () => void;
  preview: WorkGraphWorkflowPreview;
  sessionKey: string;
}

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
  const startContextRef = useRef({
    locale,
    failureMessage: t("execution.workflow_editor_start_failed"),
  });
  const editorRef = useRef<WorkGraphWorkflowEditorSession | null>(null);
  const closedRef = useRef(false);
  const [editor, setEditor] = useState<WorkGraphWorkflowEditorSession | null>(null);
  const [busy, setBusy] = useState(false);
  const [applying, setApplying] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const updateEditor = useCallback((next: WorkGraphWorkflowEditorSession) => {
    editorRef.current = next;
    setEditor(next);
  }, []);

  useEffect(() => {
    let active = true;
    closedRef.current = false;
    const initialPreview = initialPreviewRef.current;
    const startContext = startContextRef.current;
    void startWorkGraphWorkflowEditorApi(sessionKey, initialPreview, startContext.locale)
      .then((session) => {
        if (!active) return;
        updateEditor(session);
      })
      .catch((reason: unknown) => {
        if (active) {
          setError(getErrorMessage(reason, startContext.failureMessage));
        }
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
      const current = editorRef.current;
      if (current && !closedRef.current) {
        closedRef.current = true;
        void closeWorkGraphWorkflowEditorApi(sessionKey, current.editor_id).catch(() => undefined);
      }
    };
  }, [sessionKey, updateEditor]);

  const refreshEditor = useCallback(async () => {
    const current = editorRef.current;
    if (!current || closedRef.current) return current;
    try {
      const next = await getWorkGraphWorkflowEditorApi(sessionKey, current.editor_id);
      updateEditor(next);
      setError(null);
      return next;
    } catch (reason: unknown) {
      setError(getErrorMessage(reason, t("execution.workflow_editor_refresh_failed")));
      return current;
    }
  }, [sessionKey, t, updateEditor]);

  const handleSnapshotChange = useCallback((_snapshot: SessionSnapshotPayload) => {
    void refreshEditor();
  }, [refreshEditor]);

  const disposeEditor = useCallback(async () => {
    const current = editorRef.current;
    if (!current || closedRef.current) return;
    closedRef.current = true;
    await closeWorkGraphWorkflowEditorApi(sessionKey, current.editor_id).catch(() => undefined);
  }, [sessionKey]);

  const handleClose = useCallback(() => {
    void disposeEditor();
    onClose();
  }, [disposeEditor, onClose]);

  const handleApply = useCallback(async () => {
    const current = await refreshEditor();
    if (!current || busy || applying) return;
    setApplying(true);
    setError(null);
    try {
      const applied = await applyWorkGraphWorkflowEditorApi(
        sessionKey,
        current.editor_id,
        current.revision,
      );
      await disposeEditor();
      onApply(applied);
    } catch (reason: unknown) {
      setError(getErrorMessage(reason, t("execution.workflow_editor_apply_failed")));
    } finally {
      setApplying(false);
    }
  }, [applying, busy, disposeEditor, onApply, refreshEditor, sessionKey, t]);

  const sessionIdentity = useMemo(() => editor ? {
    agent_id: editor.agent_id,
    chat_type: "dm" as const,
    session_key: editor.session_key,
  } : null, [editor]);
  const agent = useMemo(
    () => editor
      ? agents.find((item) => item.agent_id === editor.agent_id) ?? null
      : null,
    [agents, editor],
  );
  const currentPreview = editor?.preview ?? preview;

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="z-[10000]"
        labelledBy="workgraph-metadata-editor-title"
        onClose={handleClose}
      >
        <UiDialogShell className="pointer-events-auto h-[min(720px,86vh)] max-h-[86vh]" size="wide">
          <UiDialogHeader
            icon={<MessageSquareText className="h-4 w-4" />}
            iconClassName="text-(--primary)"
            onClose={handleClose}
            subtitle={t("execution.workflow_editor_subtitle")}
            title={t("execution.workflow_editor_title")}
            titleId="workgraph-metadata-editor-title"
          />
          <UiDialogBody className="grid min-h-0 flex-1 overflow-hidden p-0 md:grid-cols-[minmax(320px,0.9fr)_minmax(0,1.1fr)]">
            <div className="relative flex min-h-0 min-w-0 flex-col overflow-hidden bg-(--surface-canvas-background)">
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
                <div className="grid min-h-0 flex-1 place-items-center px-5 text-center text-xs leading-5 text-(--destructive)">
                  {error ?? t("execution.workflow_editor_start_failed")}
                </div>
              )}
            </div>

            <div className="flex min-h-0 min-w-0 flex-col overflow-hidden border-l border-(--divider-subtle-color) bg-(--surface-muted-background)">
              <div className="shrink-0 border-b border-(--divider-subtle-color) px-4 py-3">
                <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
                  <h3 className="truncate text-sm font-semibold text-(--text-strong)">{currentPreview.title}</h3>
                  <code className="rounded-[6px] bg-(--surface-control-background) px-1.5 py-0.5 text-[11px] text-(--text-soft)">/{currentPreview.slash_name}</code>
                </div>
                <p className="mt-1 line-clamp-2 text-xs leading-5 text-(--text-muted)">{currentPreview.description}</p>
              </div>
              <div className="min-h-0 flex-1 p-3">
                <NamedWorkGraphSketch
                  key={editor?.revision ?? 0}
                  className="h-full min-h-0"
                  dependencies={currentPreview.dependencies}
                  nodes={currentPreview.nodes}
                />
              </div>
            </div>
          </UiDialogBody>
          <UiDialogFooter className="items-center justify-between gap-3">
            <span className="min-w-0 flex-1 truncate text-xs text-(--destructive)" role={error ? "alert" : undefined}>
              {error}
            </span>
            <div className="flex shrink-0 items-center gap-2">
              <button className={getDialogActionClassName("default", "compact")} type="button" onClick={handleClose}>
                {t("common.cancel")}
              </button>
              <button
                className={getDialogActionClassName("primary", "compact")}
                disabled={!editor || busy || applying}
                type="button"
                onClick={() => void handleApply()}
              >
                {applying ? <LoaderCircle className="h-3.5 w-3.5 animate-spin" /> : null}
                {t("execution.workflow_editor_apply")}
              </button>
            </div>
          </UiDialogFooter>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}
