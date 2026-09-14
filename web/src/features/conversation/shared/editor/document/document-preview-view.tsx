// INPUT: Parsed DOCX status, host refs, scale and file actions.
// OUTPUT: Shared state and persistent render/style hosts; loading/error content stays hidden and inert.
// POS: DOCX presentation; no fetching or parsing.
import type { CSSProperties, RefObject } from "react";

import { cn } from "@/shared/ui/class-name";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";
import { OfficePreviewFailureState } from "../office-preview-fallbacks";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import type { DocumentPreviewStatus } from "./document-preview-model";

const DOCUMENT_PREVIEW_STYLES = `
  .nexus-docx-preview-host .nexus-docx-preview-wrapper {
    align-items: center;
    background: transparent !important;
    box-sizing: border-box;
    display: flex;
    flex-direction: column;
    gap: 18px;
    max-width: none;
    min-width: 0;
    padding: 0 !important;
    zoom: var(--docx-preview-scale, 1);
  }

  .nexus-docx-preview-host section.nexus-docx-preview {
    background: #ffffff;
    box-shadow: 0 18px 36px rgba(15, 23, 42, 0.14);
    box-sizing: border-box;
    color: #111827;
    overflow: hidden;
  }

  .nexus-docx-preview-host section.nexus-docx-preview table {
    border-collapse: collapse;
  }

  .nexus-docx-preview-host section.nexus-docx-preview img,
  .nexus-docx-preview-host section.nexus-docx-preview svg {
    height: auto !important;
    max-width: 100% !important;
    object-fit: contain;
  }
`;

interface DocumentPreviewViewProps {
  agentId: string;
  containerRef: RefObject<HTMLDivElement | null>;
  fileName: string;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string;
  previewScale: number;
  retryPreview: () => void;
  status: DocumentPreviewStatus;
  styleContainerRef: RefObject<HTMLDivElement | null>;
  viewportRef: RefObject<HTMLDivElement | null>;
}

export function DocumentPreviewView({
  agentId,
  containerRef,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
  previewScale,
  retryPreview,
  status,
  styleContainerRef,
  viewportRef,
}: DocumentPreviewViewProps) {
  return (
    <>
      <DocumentPreviewHeader
        agentId={agentId}
        fileName={fileName}
        isPreviewFocused={isPreviewFocused}
        onTogglePreviewFocus={onTogglePreviewFocus}
        path={path}
      />
      <DocumentPreviewViewport
        containerRef={containerRef}
        previewScale={previewScale}
        retryPreview={retryPreview}
        status={status}
        styleContainerRef={styleContainerRef}
        viewportRef={viewportRef}
      />
    </>
  );
}

interface DocumentPreviewHeaderProps {
  agentId: string;
  fileName: string;
  isPreviewFocused: boolean;
  onTogglePreviewFocus: () => void;
  path: string;
}

function DocumentPreviewHeader({
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
}: DocumentPreviewHeaderProps) {
  return (
    <WorkspaceFilePreviewHeader
      actions={(
        <>
          <WorkspaceFileDownloadButton
            agentId={agentId}
            fileName={fileName}
            path={path}
          />
          <WorkspaceFilePreviewFocusButton
            isPreviewFocused={isPreviewFocused}
            onTogglePreviewFocus={onTogglePreviewFocus}
          />
        </>
      )}
      title={fileName}
    />
  );
}

interface DocumentPreviewViewportProps {
  containerRef: RefObject<HTMLDivElement | null>;
  previewScale: number;
  retryPreview: () => void;
  status: DocumentPreviewStatus;
  styleContainerRef: RefObject<HTMLDivElement | null>;
  viewportRef: RefObject<HTMLDivElement | null>;
}

function DocumentPreviewViewport({
  containerRef,
  previewScale,
  retryPreview,
  status,
  styleContainerRef,
  viewportRef,
}: DocumentPreviewViewportProps) {
  const hostStyle = {
    "--docx-preview-scale": String(previewScale),
  } as CSSProperties;

  return (
    <div
      ref={viewportRef}
      className="soft-scrollbar relative min-h-0 flex-1 overflow-auto bg-[var(--surface-panel-subtle-background)] p-5"
    >
      <style>{DOCUMENT_PREVIEW_STYLES}</style>
      <div ref={styleContainerRef} aria-hidden="true" className="contents" />
      {status.state === "error" ? <OfficePreviewFailureState kind="document" onRetry={retryPreview} /> : null}
      <div
        ref={containerRef}
        aria-hidden={status.state !== "loaded"}
        inert={status.state !== "loaded"}
        className={cn(
          "nexus-docx-preview-host mx-auto flex min-h-full w-full min-w-0 justify-center",
          status.state === "error" ? "hidden" : status.state === "loaded" ? "opacity-100" : "opacity-0",
        )}
        style={hostStyle}
      />
      {status.state === "loading" ? (
        <WorkspaceFilePreviewLoading className="pointer-events-none absolute inset-0" />
      ) : null}
    </div>
  );
}
