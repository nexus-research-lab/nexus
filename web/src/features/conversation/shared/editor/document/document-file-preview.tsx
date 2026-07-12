"use client";

import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { DocumentPreviewView } from "./document-preview-view";
import { useDocumentPreview } from "./use-document-preview";

export function DocumentFilePreview({
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: WorkspaceFilePreviewProps) {
  const preview = useDocumentPreview({ agentId, path });

  return (
    <DocumentPreviewView
      agentId={agentId}
      fileName={fileName}
      isPreviewFocused={isPreviewFocused}
      onTogglePreviewFocus={onTogglePreviewFocus}
      path={path}
      {...preview}
    />
  );
}
