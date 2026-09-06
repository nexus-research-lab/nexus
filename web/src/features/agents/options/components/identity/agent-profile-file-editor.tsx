// INPUT: exact Agent 的 AGENTS.md 文件 scope 与资料页保存确认。
// OUTPUT: 具名公共源码编辑器与 Agent 隔离的忙碌保存确认，保留较新的草稿。
// POS: Agent 身份页适配层；不绕过 workspace 文本编辑器的可靠性边界。
"use client";

import { useCallback, useEffect, useId, useLayoutEffect, useRef, useState } from "react";

import { buildTextFileEditorPresentation } from "@/features/conversation/shared/editor/text/text-file-editor-model";
import { TextFileEditorBody } from "@/features/conversation/shared/editor/text/text-file-editor-body";
import { TextFileEditorReliability } from "@/features/conversation/shared/editor/text/text-file-editor-reliability";
import { useTextFileEditor } from "@/features/conversation/shared/editor/text/use-text-file-editor";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import { UiField } from "@/shared/ui/form/form-control";

const AGENT_PROFILE_FILE_PATH = "AGENTS.md";

interface AgentProfileFileEditorProps {
  agentId: string;
  label: string;
}

export function AgentProfileFileEditor(props: AgentProfileFileEditorProps) {
  return <AgentProfileFileEditorScope key={props.agentId} {...props} />;
}

function AgentProfileFileEditorScope({
  agentId,
  label,
}: AgentProfileFileEditorProps) {
  const { t } = useI18n();
  const editor = useTextFileEditor({
    agentId,
    fallbackLoadError: t("workspace_file.load_failed_fallback"),
    fallbackSaveError: t("workspace_file.save_failed_fallback"),
    path: AGENT_PROFILE_FILE_PATH,
  });
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const [confirmBusy, setConfirmBusy] = useState(false);
  const editorId = useId();
  const mountedRef = useRef(true);
  const submittingRef = useRef(false);
  const currentEditorRef = useRef(editor);
  useLayoutEffect(() => { currentEditorRef.current = editor; }, [editor]);
  useEffect(() => {
    mountedRef.current = true;
    return () => { mountedRef.current = false; };
  }, []);
  const revisionReady = Boolean(editor.revision);
  const saveBlocked = Boolean(
    editor.saveIssue
    && editor.saveIssue.kind !== "not_applied"
    && editor.saveIssue.kind !== "retry_ready",
  );
  const presentation = buildTextFileEditorPresentation({
    commandBusy: editor.isSaving || editor.isReconciling,
    fileType: "markdown",
    isAvailable: editor.hasLoadedContent,
    isDirty: editor.isDirty,
    isEditing: editor.isEditing,
    isExternalWriting: editor.isExternalWriting,
    isLoading: editor.isLoading,
    isSaving: editor.isSaving,
    liveState: editor.liveState,
    revisionReady,
    saveBlocked,
    translate: t,
  });
  const isBusy = confirmBusy || !editor.hasLoadedContent
    || !revisionReady
    || editor.isLoading
    || editor.isExternalWriting
    || editor.isSaving
    || editor.isReconciling
    || saveBlocked;

  useEffect(() => {
    if (!confirmBusy && (!editor.hasLoadedContent || !revisionReady || saveBlocked)) setIsConfirmOpen(false);
  }, [confirmBusy, editor.hasLoadedContent, revisionReady, saveBlocked]);

  const handleEditAction = useCallback(() => {
    if (isBusy) {
      return;
    }
    if (!editor.isEditing) {
      editor.setIsEditing(true);
      return;
    }
    if (!editor.isDirty) {
      editor.setIsEditing(false);
      return;
    }
    setIsConfirmOpen(true);
  }, [editor, isBusy]);

  const handleConfirmSave = useCallback(async () => {
    if (isBusy || submittingRef.current || !isConfirmOpen) return;
    const submittedDraft = editor.draftContent;
    submittingRef.current = true;
    setConfirmBusy(true);
    try {
      const didSave = await editor.save();
      if (!mountedRef.current) return;
      setIsConfirmOpen(false);
      const current = currentEditorRef.current;
      if (didSave && (current.draftContent === submittedDraft || !current.isDirty)) {
        current.setIsEditing(false);
      }
    } finally {
      submittingRef.current = false;
      if (mountedRef.current) setConfirmBusy(false);
    }
  }, [editor, isBusy, isConfirmOpen]);

  const handleCancelConfirmation = useCallback(() => {
    if (!submittingRef.current) setIsConfirmOpen(false);
  }, []);

  return (
    <UiField className="flex min-h-0 min-w-0 flex-1 flex-col gap-2"
      htmlFor={presentation.bodyMode === "editing" ? editorId : undefined} label={label}
      labelAction={(
        <UiButton
          disabled={isBusy}
          onClick={handleEditAction}
          size="sm"
          tone={editor.isEditing ? "primary" : "default"}
          variant="surface"
        >
          {editor.isEditing ? t("common.save") : t("common.edit")}
        </UiButton>
      )}>

      <div
        className={cn(
          "min-h-0 min-w-0 flex-1 overflow-hidden surface-radius-lg",
          editor.isEditing
            ? "border border-(--modal-input-border) bg-(--modal-input-background)"
            : "dialog-input",
        )}
      >
        <TextFileEditorBody
          agentId={agentId}
          content={editor.displayContent}
          editorId={editorId}
          editorLabel={label}
          exitEditingOnBlur={false}
          fileName={AGENT_PROFILE_FILE_PATH}
          fileType="markdown"
          isLoading={editor.isLoading && !editor.hasLoadedContent}
          isStreaming={editor.isExternalWriting}
          mode={presentation.bodyMode}
          setContent={editor.setDraftContent}
          setIsEditing={editor.setIsEditing}
        />
      </div>

      <TextFileEditorReliability
        hasLoadedContent={editor.hasLoadedContent}
        isLoading={editor.isLoading}
        isReconciling={editor.isReconciling}
        isSaving={editor.isSaving}
        onAdoptLatest={editor.adoptLatest}
        onLoadLatest={() => void editor.loadContent()}
        onOverwrite={() => void editor.overwriteConflict()}
        onReconcile={() => void editor.reconcileSave()}
        onRetrySave={() => void editor.save()}
        resourceFailure={editor.resourceFailure}
        revisionReady={revisionReady}
        saveIssue={editor.saveIssue}
      />

      <ConfirmDialog
        busy={confirmBusy || editor.isSaving || editor.isReconciling}
        cancelText={t("agent_options.identity.profile_save_confirm_cancel")}
        confirmText={t("agent_options.identity.profile_save_confirm_action")}
        isOpen={isConfirmOpen}
        message={t("agent_options.identity.profile_save_confirm_message")}
        onCancel={handleCancelConfirmation}
        onConfirm={() => {
          void handleConfirmSave();
        }}
        title={t("agent_options.identity.profile_save_confirm_title")}
      />
    </UiField>
  );
}
