// INPUT: exact Agent 的 AGENTS.md 文件 scope 与资料页编辑/保存动作。
// OUTPUT: 具名公共源码编辑器与 Agent 隔离的直接保存，保存期间继续编辑时保留较新的草稿。
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
  const [submitBusy, setSubmitBusy] = useState(false);
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
  const isBusy = submitBusy || !editor.hasLoadedContent
    || !revisionReady
    || editor.isLoading
    || editor.isExternalWriting
    || editor.isSaving
    || editor.isReconciling
    || saveBlocked;

  // 点「保存」即提交：版本冲突与结果未知仍由文本编辑器的 revision 与对账处理，不再二次确认。
  const handleSave = useCallback(async () => {
    if (isBusy || submittingRef.current) return;
    const submittedDraft = editor.draftContent;
    submittingRef.current = true;
    setSubmitBusy(true);
    try {
      const didSave = await editor.save();
      if (!mountedRef.current) return;
      const current = currentEditorRef.current;
      if (didSave && (current.draftContent === submittedDraft || !current.isDirty)) {
        current.setIsEditing(false);
      }
    } finally {
      submittingRef.current = false;
      if (mountedRef.current) setSubmitBusy(false);
    }
  }, [editor, isBusy]);

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
    void handleSave();
  }, [editor, handleSave, isBusy]);

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

    </UiField>
  );
}
