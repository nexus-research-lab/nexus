// INPUT: Exact file/owner lifecycle, native media load/error events and existing file actions.
// OUTPUT: Shared native-media lifecycle; images retry known failures, PDF offers explicit reload.
// POS: Media presentation; preserves native sandboxes and file commands, without guessing PDF success.
"use client";

import { FileWarning, RefreshCw } from "lucide-react";

import {
  getWorkspaceFilePreviewUrl,
} from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { WORKSPACE_PANEL_HEADER_ICON_CLASS } from "@/shared/ui/workspace/surface/workspace-header-layout";
import { WorkspaceFilePreviewLoading } from "../workspace-file-preview-loading";
import {
  WorkspaceFileDownloadButton,
  WorkspaceFilePreviewFocusButton,
  WorkspaceFilePreviewHeader,
  WorkspaceFileToolbarButton,
} from "../workspace-file-preview-chrome";
import type { WorkspaceFilePreviewProps } from "../workspace-file-preview-types";
import { useNativeMediaPreview } from "./use-native-media-preview";

export function PdfPreview(props: WorkspaceFilePreviewProps) {
  return <NativeMediaPreview {...props} kind="pdf" />;
}

export function ImagePreview(props: WorkspaceFilePreviewProps) {
  return <NativeMediaPreview {...props} kind="image" />;
}

function NativeMediaPreview({
  agentId,
  path,
  fileName,
  isPreviewFocused,
  onTogglePreviewFocus,
  kind,
}: WorkspaceFilePreviewProps & { kind: "image" | "pdf" }) {
  const { t } = useI18n();
  const { loadState, previewKey, onLoad, onError, reloadPreview } = useNativeMediaPreview(agentId, path);
  const previewUrl = getWorkspaceFilePreviewUrl(agentId, path);

  return (
    <>
      <WorkspaceFilePreviewHeader
        actions={(
          <>
            <WorkspaceFileDownloadButton agentId={agentId} fileName={fileName} path={path} />
            {kind === "pdf" ? (
              <WorkspaceFileToolbarButton onClick={reloadPreview} title={t("workspace_file.reload_preview")}>
                <RefreshCw aria-hidden="true" className={WORKSPACE_PANEL_HEADER_ICON_CLASS} />
              </WorkspaceFileToolbarButton>
            ) : null}
            <WorkspaceFilePreviewFocusButton
              isPreviewFocused={isPreviewFocused}
              onTogglePreviewFocus={onTogglePreviewFocus}
            />
          </>
        )}
        title={fileName}
      />
      <div className="relative flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden bg-[var(--surface-panel-subtle-background)]">
        {kind === "image" && loadState === "error" ? (
          <ImagePreviewFailure onRetry={reloadPreview} />
        ) : kind === "pdf" ? (
          <iframe
            className="block h-full min-h-0 w-full border-0"
            key={previewKey}
            // iframe load only ends the pending indicator; it cannot prove PDF success.
            // https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Elements/iframe#error_and_load_event_behavior
            onLoad={onLoad}
            sandbox="allow-downloads allow-same-origin"
            src={previewUrl}
            title={fileName}
          />
        ) : (
          <div className="flex min-h-0 min-w-0 flex-1 items-center justify-center p-6">
            <img
              className="max-h-full max-w-full radius-control-sm object-contain"
              key={previewKey}
              src={previewUrl}
              alt={fileName}
              onLoad={onLoad}
              onError={onError}
            />
          </div>
        )}
        {loadState === "loading" ? <WorkspaceFilePreviewLoading className="pointer-events-none absolute inset-0" /> : null}
      </div>
    </>
  );
}

function ImagePreviewFailure({
  onRetry,
}: {
  onRetry: () => void;
}) {
  const { t } = useI18n();
  return (
    <div className="soft-scrollbar min-h-0 w-full flex-1 overflow-auto overscroll-contain p-4">
      <UiResourceState
        className="mx-auto min-h-0 w-full max-w-lg py-5"
        impact={t("workspace_file.media_preview_failed_impact")}
        primaryAction={{
          label: t("workspace_file.retry_preview"),
          onClick: onRetry,
        }}
        size="sm"
        state="error"
        title={t("workspace_file.image_preview_failed")}
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
