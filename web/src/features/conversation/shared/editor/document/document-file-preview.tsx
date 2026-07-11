"use client";

import { ConversationResizeHandle } from "../conversation-resize-handle";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { DocumentPreviewView } from "./document-preview-view";
import { useDocumentPreview } from "./use-document-preview";

export function DocumentFilePreview({
  agentId,
  embedded,
  fileName,
  isPreviewFocused,
  onResizeStart,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const preview = useDocumentPreview({ agentId, path });

  return (
    <>
      {!embedded ? (
        <ConversationResizeHandle
          ariaLabel="调整编辑器宽度"
          className="flex"
          onMouseDown={onResizeStart}
        />
      ) : null}
      <DocumentPreviewView
        agentId={agentId}
        embedded={embedded}
        fileName={fileName}
        isPreviewFocused={isPreviewFocused}
        onTogglePreviewFocus={onTogglePreviewFocus}
        path={path}
        {...preview}
      />
    </>
  );
}
