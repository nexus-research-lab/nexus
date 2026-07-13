"use client";

import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { TextFileEditorBody } from "./text-file-editor-body";
import { TextFileEditorHeader } from "./text-file-editor-header";
import { buildTextFileEditorPresentation } from "./text-file-editor-model";
import { useTextFileEditor } from "./use-text-file-editor";

export function TextFileEditor({
  agentId,
  fileName,
  fileType,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps & { fileType: WorkspaceFilePreviewKind }) {
  const editor = useTextFileEditor({ agentId, path });
  const presentation = buildTextFileEditorPresentation({
    fileType,
    isDirty: editor.isDirty,
    isEditing: editor.isEditing,
    isExternalWriting: editor.isExternalWriting,
    isSaving: editor.isSaving,
    liveState: editor.liveState,
  });

  return (
    <>
      <TextFileEditorHeader
        agentId={agentId}
        fileName={fileName}
        fileType={fileType}
        isPreviewFocused={isPreviewFocused}
        onSave={() => void editor.save()}
        onToggleEditing={editor.toggleEditing}
        onTogglePreviewFocus={onTogglePreviewFocus}
        path={path}
        presentation={presentation}
      />
      {editor.error ? (
        <div className="px-4 py-3 text-sm text-destructive">
          {editor.error}
        </div>
      ) : null}
      <TextFileEditorBody
        content={editor.draftContent}
        fileName={fileName}
        fileType={fileType}
        isLoading={editor.isLoading}
        isStreaming={editor.isExternalWriting}
        mode={presentation.bodyMode}
        setContent={editor.setDraftContent}
        setIsEditing={editor.setIsEditing}
      />
    </>
  );
}
