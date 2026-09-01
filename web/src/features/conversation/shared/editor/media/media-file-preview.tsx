"use client";

import { useCallback, useState } from "react";
import {
  Eye,
  EyeOff,
  FileWarning,
  LoaderCircle,
} from "lucide-react";

import {
  getWorkspaceFilePreviewUrl,
} from "@/lib/api/agent/agent-api";
import { getWorkspaceFileExternalActionCopy } from "@/lib/workspace-file-action";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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
  const { t } = useI18n();
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
        meta={(
          loadState === "error" ? (
            <span className="flex items-center gap-1 text-destructive">
              <EyeOff className="h-3 w-3" />
              {t("workspace_file.preview_failed_status")}
            </span>
          ) : loadState === "loaded" ? (
            <span className="flex items-center gap-1 text-(--success)">
              <Eye className="h-3 w-3" />
              {t("workspace_file.preview_loaded")}
            </span>
          ) : (
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              {t("workspace_file.preview_loading")}
            </span>
          )
        )}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)]">
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
  const { t } = useI18n();
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
        meta={(
          loadState === "error" ? (
            <span className="flex items-center gap-1 text-destructive">
              <EyeOff className="h-3 w-3" />
              {t("workspace_file.preview_failed_status")}
            </span>
          ) : loadState === "loaded" ? (
            <span className="flex items-center gap-1 text-(--success)">
              <Eye className="h-3 w-3" />
              {t("workspace_file.preview_loaded")}
            </span>
          ) : (
            <span className="flex items-center gap-1">
              <LoaderCircle className="h-3 w-3 animate-spin" />
              {t("workspace_file.preview_loading")}
            </span>
          )
        )}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-6">
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
  const actionDescription = fileActionCopy.mode === "reveal"
    ? "在文件夹中显示此文件"
    : "获取此文件";
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
        meta={(
          <span className="flex items-center gap-1">
            <FileWarning className="h-3 w-3" />
            此文件类型不支持预览
          </span>
        )}
        title={fileName}
      />

      <div className="min-h-0 flex-1 overflow-hidden bg-[var(--surface-panel-subtle-background)] p-8">
        <div className="m-auto max-w-xs text-center">
          <div className="mx-auto mb-4 flex h-16 w-16 items-center justify-center surface-radius-md border border-(--surface-panel-subtle-border) bg-(--card-default-background)">
            <FileWarning className="h-8 w-8 text-(--icon-muted)" />
          </div>
          <p className="text-sm font-medium text-(--text-strong)">不支持预览此文件</p>
          <p className="mt-2 text-xs leading-5 text-(--text-soft)">
            当前预览仅支持文本、PDF、图片、xlsx、docx 和 pptx 文件。您可以点击上方"{fileActionCopy.label}"按钮{actionDescription}。
          </p>
        </div>
      </div>
    </>
  );
}
