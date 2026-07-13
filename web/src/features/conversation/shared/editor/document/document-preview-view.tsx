import type { CSSProperties, ReactNode, RefObject } from "react";
import { Eye, FileText, FileWarning, LoaderCircle } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
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
        status={status}
      />
      <DocumentPreviewViewport
        containerRef={containerRef}
        previewScale={previewScale}
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
  status: DocumentPreviewStatus;
}

function DocumentPreviewHeader({
  agentId,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  path,
  status,
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
      meta={(
        <>
          <span className="flex items-center gap-1">
            <FileText className="h-3 w-3" />
            docx 预览
          </span>
          <DocumentPreviewStatusMeta status={status} />
        </>
      )}
      title={fileName}
    />
  );
}

function DocumentPreviewStatusMeta({
  status,
}: { status: DocumentPreviewStatus }) {
  const statusViews = {
    error: (
      <span className="flex items-center gap-1 text-destructive">
        <FileWarning className="h-3 w-3" />
        加载失败
      </span>
    ),
    loaded: (
      <span className="flex items-center gap-1 text-(--success)">
        <Eye className="h-3 w-3" />
        已加载
      </span>
    ),
    loading: (
      <span className="flex items-center gap-1">
        <LoaderCircle className="h-3 w-3 animate-spin" />
        {status.state === "loading" ? status.message : "加载中"}
      </span>
    ),
  } satisfies Record<DocumentPreviewStatus["state"], ReactNode>;

  return statusViews[status.state];
}

interface DocumentPreviewViewportProps {
  containerRef: RefObject<HTMLDivElement | null>;
  previewScale: number;
  status: DocumentPreviewStatus;
  styleContainerRef: RefObject<HTMLDivElement | null>;
  viewportRef: RefObject<HTMLDivElement | null>;
}

function DocumentPreviewViewport({
  containerRef,
  previewScale,
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
      {status.state === "error" ? (
        <DocumentPreviewError message={status.message} />
      ) : (
        <div
          ref={containerRef}
          className={cn(
            "nexus-docx-preview-host mx-auto flex min-h-full w-full min-w-0 justify-center",
            status.state === "loaded" ? "opacity-100" : "opacity-0",
          )}
          style={hostStyle}
        />
      )}
      {status.state === "loading" ? (
        <DocumentPreviewLoading message={status.message} />
      ) : null}
    </div>
  );
}

function DocumentPreviewError({ message }: { message: string }) {
  return (
    <div className="flex h-full min-h-[240px] items-center justify-center text-center">
      <div className="max-w-sm">
        <FileWarning className="mx-auto h-12 w-12 text-(--icon-muted)" />
        <p className="mt-4 text-sm font-medium text-(--text-strong)">
          docx 预览失败
        </p>
        <p className="mt-2 text-xs leading-5 text-(--text-soft)">{message}</p>
      </div>
    </div>
  );
}

function DocumentPreviewLoading({ message }: { message: string }) {
  return (
    <div className="pointer-events-none absolute inset-x-0 top-24 flex justify-center">
      <div className="inline-flex items-center gap-2 rounded-full border border-(--divider-subtle-color) bg-(--surface-panel-background) px-3 py-1.5 text-xs text-(--text-muted) shadow-sm">
        <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
        <span>{message}</span>
      </div>
    </div>
  );
}
