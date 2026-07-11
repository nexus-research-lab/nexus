"use client";

import { ConversationResizeHandle } from "../conversation-resize-handle";
import type { WorkspaceFilePreviewKind } from "../workspace-file-preview-kind";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { TextFileEditorBody } from "./text-file-editor-body";
import { TextFileEditorHeader } from "./text-file-editor-header";
import { buildTextFileEditorPresentation } from "./text-file-editor-model";
import { useTextFileEditor } from "./use-text-file-editor";

export function TextFileEditor({
  agentId,
  embedded,
  fileName,
  fileType,
  isPreviewFocused,
  onResizeStart,
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
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}
      <TextFileEditorHeader
        agentId={agentId}
        embedded={embedded}
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
