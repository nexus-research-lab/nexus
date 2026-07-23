"use client";

import { useCallback, useState } from "react";

import { buildTextFileEditorPresentation } from "@/features/conversation/shared/editor/text/text-file-editor-model";
import { TextFileEditorBody } from "@/features/conversation/shared/editor/text/text-file-editor-body";
import { useTextFileEditor } from "@/features/conversation/shared/editor/text/use-text-file-editor";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";

const AGENT_PROFILE_FILE_PATH = "AGENTS.md";

interface AgentProfileFileEditorProps {
  agentId: string;
  label: string;
}

export function AgentProfileFileEditor({
  agentId,
  label,
}: AgentProfileFileEditorProps) {
  const { t } = useI18n();
  const editor = useTextFileEditor({
    agentId,
    path: AGENT_PROFILE_FILE_PATH,
  });
  const [isConfirmOpen, setIsConfirmOpen] = useState(false);
  const presentation = buildTextFileEditorPresentation({
    fileType: "markdown",
    isDirty: editor.isDirty,
    isEditing: editor.isEditing,
    isExternalWriting: editor.isExternalWriting,
    isSaving: editor.isSaving,
    liveState: editor.liveState,
  });
  const isBusy = editor.isLoading || editor.isExternalWriting || editor.isSaving;

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
    if (editor.isSaving) {
      return;
    }
    const didSave = await editor.save();
    if (!didSave) {
      return;
    }
    setIsConfirmOpen(false);
    editor.setIsEditing(false);
  }, [editor]);

  const handleCancelConfirmation = useCallback(() => {
    setIsConfirmOpen(false);
  }, []);

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col gap-2">
      <div className="flex shrink-0 items-center justify-between gap-3">
        <label className="text-[11px] font-semibold text-(--text-muted)">
          {label}
        </label>
        <UiButton
          disabled={isBusy}
          onClick={handleEditAction}
          size="sm"
          tone={editor.isEditing ? "primary" : "default"}
          variant="surface"
        >
          {editor.isEditing ? t("common.save") : t("common.edit")}
        </UiButton>
      </div>

      <div
        className={cn(
          "min-h-0 min-w-0 flex-1 overflow-hidden surface-radius-lg",
          editor.isEditing
            ? "border border-(--modal-input-border) bg-(--modal-input-background)"
            : "dialog-input",
        )}
      >
        <TextFileEditorBody
          content={editor.draftContent}
          exitEditingOnBlur={false}
          fileName={AGENT_PROFILE_FILE_PATH}
          fileType="markdown"
          isLoading={editor.isLoading}
          isStreaming={editor.isExternalWriting}
          mode={presentation.bodyMode}
          setContent={editor.setDraftContent}
          setIsEditing={editor.setIsEditing}
        />
      </div>

      {editor.error ? (
        <p className="shrink-0 text-xs leading-5 text-(--destructive)">
          {editor.error}
        </p>
      ) : null}

      <ConfirmDialog
        cancelText={t("agent_options.identity.profile_save_confirm_cancel")}
        confirmText={t("agent_options.identity.profile_save_confirm_action")}
        isOpen={isConfirmOpen}
        message={editor.error ?? t("agent_options.identity.profile_save_confirm_message")}
        onCancel={handleCancelConfirmation}
        onConfirm={() => {
          void handleConfirmSave();
        }}
        title={t("agent_options.identity.profile_save_confirm_title")}
      />
    </div>
  );
}
