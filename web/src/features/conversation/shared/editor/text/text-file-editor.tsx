// INPUT: exact Agent/path 文件预览 scope、焦点布局与当前语言。
// OUTPUT: revision 保护的文本编辑器、工具栏和完整异常恢复状态。
// POS: 通用文本文件编辑入口；业务可靠性由控制器持有，视图不猜写入结果。
"use client";

import { useI18n } from "@/shared/i18n/i18n-context";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { TextFileEditorBody } from "./text-file-editor-body";
import { TextFileEditorHeader } from "./text-file-editor-header";
import { buildTextFileEditorPresentation } from "./text-file-editor-model";
import { TextFileEditorReliability } from "./text-file-editor-reliability";
import { useTextFileEditor } from "./use-text-file-editor";

export function TextFileEditor({
  agentId,
  fileName,
  fileType,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps & { fileType: WorkspaceFilePreviewKind }) {
  const { t } = useI18n();
  const editor = useTextFileEditor({
    agentId,
    fallbackLoadError: t("workspace_file.load_failed_fallback"),
    fallbackSaveError: t("workspace_file.save_failed_fallback"),
    path,
  });
  const revisionReady = Boolean(editor.revision);
  const saveBlocked = Boolean(
    editor.saveIssue
    && editor.saveIssue.kind !== "not_applied"
    && editor.saveIssue.kind !== "retry_ready",
  );
  const presentation = buildTextFileEditorPresentation({
    commandBusy: editor.isSaving || editor.isReconciling,
    fileType,
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

  return (
    <>
      <TextFileEditorHeader
        agentId={agentId}
        fileName={fileName}
        isPreviewFocused={isPreviewFocused}
        onSave={() => void editor.save()}
        onToggleEditing={editor.toggleEditing}
        onTogglePreviewFocus={onTogglePreviewFocus}
        path={path}
        presentation={presentation}
      />
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
      <TextFileEditorBody
        content={editor.displayContent}
        fileName={fileName}
        fileType={fileType}
        isLoading={editor.isLoading && !editor.hasLoadedContent}
        isStreaming={editor.isExternalWriting}
        mode={presentation.bodyMode}
        setContent={editor.setDraftContent}
        setIsEditing={editor.setIsEditing}
      />
    </>
  );
}
