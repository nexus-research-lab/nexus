/**
 * INPUT: 当前选中的本地图片或文本附件与关闭动作。
 * OUTPUT: 支持遮罩/Escape 关闭、焦点恢复的大图灯箱或只读文本预览。
 * POS: Composer 草稿附件的模态预览边界。
 */
"use client";

import { useEffect, useState } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogCloseButton,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { ComposerLocalAttachment } from "./composer-local-attachment-model";
import { useComposerLocalFileUrl } from "./use-composer-local-file-url";

const MAX_TEXT_PREVIEW_BYTES = 512 * 1024;
const IMAGE_PREVIEW_TITLE_ID = "composer-image-preview-title";
const TEXT_PREVIEW_TITLE_ID = "composer-text-preview-title";

interface ComposerAttachmentPreviewDialogProps {
  attachment: ComposerLocalAttachment | null;
  onClose: () => void;
}

interface TextPreviewState {
  content: string;
  isTruncated: boolean;
  status: "loading" | "ready" | "error";
}

export function ComposerAttachmentPreviewDialog({
  attachment,
  onClose,
}: ComposerAttachmentPreviewDialogProps) {
  if (!attachment) {
    return null;
  }
  if (attachment.kind === "image") {
    return (
      <ComposerImagePreviewDialog
        attachment={attachment}
        onClose={onClose}
      />
    );
  }
  if (attachment.kind === "text") {
    return (
      <ComposerTextPreviewDialog
        attachment={attachment}
        onClose={onClose}
      />
    );
  }
  return null;
}

function ComposerImagePreviewDialog({
  attachment,
  onClose,
}: {
  attachment: ComposerLocalAttachment;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const imageUrl = useComposerLocalFileUrl(attachment.file);
  const [imageFailed, setImageFailed] = useState(false);

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="overscroll-contain"
        inset="compact"
        labelledBy={IMAGE_PREVIEW_TITLE_ID}
        layer="dialogNested"
        onClose={onClose}
      >
        <UiDialogShell
          size="xl"
          viewport="visualPreview"
        >
          <UiDialogHeader
            appearance="plain"
            actions={
              <UiDialogCloseButton
                ariaLabel={t("composer.close_attachment_preview")}
                className="h-7 w-7"
                onClose={onClose}
              />
            }
            className="gap-2 px-3 py-1.5"
          >
            <div className="flex min-w-0 flex-1 items-center">
              <h2
                className="min-w-0 flex-1 truncate text-sm font-medium text-(--text-strong)"
                id={IMAGE_PREVIEW_TITLE_ID}
              >
                {attachment.file.name}
              </h2>
            </div>
          </UiDialogHeader>
          <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-(--surface-paper-background) p-3 sm:p-4">
            {imageFailed ? (
              <AttachmentPreviewFailure onClose={onClose} />
            ) : imageUrl ? (
              <img
                alt={attachment.file.name}
                className="max-h-full max-w-full radius-control-md object-contain shadow-(--surface-paper-shadow)"
                draggable={false}
                onError={() => setImageFailed(true)}
                src={imageUrl}
              />
            ) : (
              <p className="text-sm text-(--text-soft)">
                {t("composer.attachment_preview_loading")}
              </p>
            )}
          </div>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ComposerTextPreviewDialog({
  attachment,
  onClose,
}: {
  attachment: ComposerLocalAttachment;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const preview = useComposerTextPreview(attachment.file);

  return (
    <UiDialogPortal>
      <UiDialogBackdrop
        className="overscroll-contain"
        inset="compact"
        labelledBy={TEXT_PREVIEW_TITLE_ID}
        layer="dialogNested"
        onClose={onClose}
      >
        <UiDialogShell
          size="lg"
          viewport="documentPreview"
        >
          <UiDialogHeader
            appearance="plain"
            actions={
              <UiDialogCloseButton
                ariaLabel={t("composer.close_attachment_preview")}
                className="h-7 w-7"
                onClose={onClose}
              />
            }
            className="gap-2 px-3 py-1.5"
          >
            <div className="flex min-w-0 flex-1 items-center">
              <h2
                className="min-w-0 flex-1 truncate text-sm font-medium text-(--text-strong)"
                id={TEXT_PREVIEW_TITLE_ID}
              >
                {attachment.file.name}
              </h2>
            </div>
          </UiDialogHeader>
          <div className="flex min-h-0 flex-1 flex-col bg-(--surface-paper-background)">
            {preview.isTruncated ? (
              <p className="border-b border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) px-5 py-2 text-xs text-(--text-soft)">
                {t("composer.text_preview_truncated")}
              </p>
            ) : null}
            <ComposerTextPreviewContent onClose={onClose} preview={preview} />
          </div>
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ComposerTextPreviewContent({
  onClose,
  preview,
}: {
  onClose: () => void;
  preview: TextPreviewState;
}) {
  const { t } = useI18n();
  if (preview.status === "loading") {
    return (
      <p className="m-auto text-sm text-(--text-soft)">
        {t("composer.attachment_preview_loading")}
      </p>
    );
  }
  if (preview.status === "error") {
    return <AttachmentPreviewFailure onClose={onClose} />;
  }
  return (
    <pre className="soft-scrollbar min-h-0 flex-1 overflow-auto whitespace-pre-wrap break-words px-5 py-4 font-mono text-sm leading-6 text-(--surface-paper-foreground)">
      {preview.content || t("composer.text_preview_empty")}
    </pre>
  );
}

function AttachmentPreviewFailure({ onClose }: { onClose: () => void }) {
  const { t } = useI18n();
  return (
    <UiResourceState
      className="m-auto min-h-0 w-full max-w-md py-5"
      impact={t("composer.attachment_preview_failed_impact")}
      primaryAction={{
        label: t("composer.close_attachment_preview"),
        onClick: onClose,
      }}
      size="sm"
      state="error"
      title={t("composer.attachment_preview_failed")}
      urgency="polite"
      variant="card"
    />
  );
}

function useComposerTextPreview(file: File): TextPreviewState {
  const [preview, setPreview] = useState<TextPreviewState>({
    content: "",
    isTruncated: false,
    status: "loading",
  });

  useEffect(() => {
    let isCurrent = true;
    setPreview({
      content: "",
      isTruncated: file.size > MAX_TEXT_PREVIEW_BYTES,
      status: "loading",
    });
    void file
      .slice(0, MAX_TEXT_PREVIEW_BYTES)
      .text()
      .then((content) => {
        if (isCurrent) {
          setPreview({
            content,
            isTruncated: file.size > MAX_TEXT_PREVIEW_BYTES,
            status: "ready",
          });
        }
      })
      .catch(() => {
        if (isCurrent) {
          setPreview({
            content: "",
            isTruncated: false,
            status: "error",
          });
        }
      });
    return () => {
      isCurrent = false;
    };
  }, [file]);

  return preview;
}
