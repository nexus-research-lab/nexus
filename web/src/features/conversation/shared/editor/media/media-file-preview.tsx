// INPUT: Exact file URL, native media load/error events and existing file actions.
// OUTPUT: PDF/image previews with one shared state surface and localized unsupported-file guidance.
// POS: Media presentation; keeps native PDF sandbox, retries and file commands unchanged.
"use client";

import { useCallback, useState } from "react";
import { FileWarning } from "lucide-react";

import {
  getWorkspaceFilePreviewUrl,
} from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
} from "../workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";

export function PdfPreview({
  agentId,
  path,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
}: WorkspaceFilePreviewProps) {
  const [loadState, setLoadState] = useState<"error" | "loaded" | "loading">("loading");
  const [retryRevision, setRetryRevision] = useState(0);
  const previewUrl = getWorkspaceFilePreviewUrl(agentId, path);
  const retryPreview = useCallback(() => {
    setLoadState("loading");
    setRetryRevision((current) => current + 1);
  }, []);

  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        title={fileName}
      />

      <div className="relative min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {loadState === "error" ? (
          <MediaPreviewFailure
            onRetry={retryPreview}
            titleKey="workspace_file.pdf_preview_failed"
          />
        ) : (
          <iframe
            className="h-full w-full"
            key={retryRevision}
            onError={() => setLoadState("error")}
            onLoad={() => setLoadState("loaded")}
            sandbox="allow-downloads allow-same-origin"
            src={previewUrl}
            title={fileName}
          />
        )}
        {loadState === "loading" ? <WorkspaceFilePreviewLoading className="pointer-events-none absolute inset-0" /> : null}
      </div>
    </>
  );
}

export function ImagePreview({
  agentId,
  path,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
}: WorkspaceFilePreviewProps) {
  const [loadState, setLoadState] = useState<"error" | "loaded" | "loading">("loading");
  const [retryRevision, setRetryRevision] = useState(0);
  const previewUrl = getWorkspaceFilePreviewUrl(agentId, path);
  const retryPreview = useCallback(() => {
    setLoadState("loading");
    setRetryRevision((current) => current + 1);
  }, []);

  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        title={fileName}
      />

      <div className="relative flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-[var(--surface-panel-subtle-background)] p-6">
        {loadState === "error" ? (
          <MediaPreviewFailure
            onRetry={retryPreview}
            titleKey="workspace_file.image_preview_failed"
          />
        ) : (
          <img
            className="max-h-full max-w-full radius-control-sm object-contain"
            key={retryRevision}
            src={previewUrl}
            alt={fileName}
            onLoad={() => setLoadState("loaded")}
            onError={() => setLoadState("error")}
          />
        )}
        {loadState === "loading" ? <WorkspaceFilePreviewLoading className="pointer-events-none absolute inset-0" /> : null}
      </div>
    </>
  );
}

function MediaPreviewFailure({
  onRetry,
  titleKey,
}: {
  onRetry: () => void;
  titleKey: TranslationKey;
}) {
  const { t } = useI18n();
  return (
    <div className="flex h-full min-h-[240px] w-full items-center justify-center p-6">
      <UiResourceState
        className="min-h-0 w-full max-w-lg py-5"
        impact={t("workspace_file.media_preview_failed_impact")}
        primaryAction={{
          label: t("workspace_file.retry_preview"),
          onClick: onRetry,
        }}
        size="sm"
        state="error"
        title={t(titleKey)}
        urgency="polite"
        variant="card"
      />
    </div>
  );
}

export function BinaryFilePlaceholder({
  agentId,
  path,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
}: WorkspaceFilePreviewProps) {
  const { t } = useI18n();
  const fileActionCopy = getWorkspaceFileExternalActionCopy(t, fileName);
  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        title={fileName}
      />

      <UiResourceState
        className="min-h-0 flex-1"
        description={t(fileActionCopy.mode === "reveal"
          ? "workspace_file.unsupported_preview_reveal" : "workspace_file.unsupported_preview_download")}
        icon={<FileWarning aria-hidden className="h-8 w-8 text-(--icon-muted)" />}
        size="sm" state="empty" variant="plain"
        title={t("workspace_file.unsupported_preview_title")}
      />
    </>
  );
}
