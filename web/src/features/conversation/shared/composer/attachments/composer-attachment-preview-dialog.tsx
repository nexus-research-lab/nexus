/**
 * INPUT: 当前选中的本地图片或文本附件与关闭动作。
 * OUTPUT: 单一具名模态外壳、按附件隔离的图片/有界文本预览与共享滚动状态面。
 * POS: Composer 草稿预览边界；标题/关闭/焦点归 Dialog，源码与视口样式归 shared/ui。
 */
"use client";

import { useEffect } from "react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { cn } from "@/shared/ui/class-name";
import { UI_SOURCE_TEXT_CLASS_NAME } from "@/shared/ui/form/source-text-styles";
import { UI_PREVIEW_VIEWPORT_CLASS_NAME } from "@/shared/ui/layout/preview-viewport-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiDialogBackdrop,
  UiDialogHeader,
  UiDialogPortal,
  UiDialogShell,
} from "@/shared/ui/dialog/dialog";
import { UiResourceState } from "@/shared/ui/display/resource-state";

import type { ComposerLocalAttachment } from "./composer-local-attachment-model";
import { useComposerLocalFileUrl } from "./use-composer-local-file-url";

const MAX_TEXT_PREVIEW_BYTES = 512 * 1024;

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
  const { t } = useI18n();
  if (!attachment || attachment.kind === "file") return null;
  const isImage = attachment.kind === "image";
  return (
    <UiDialogPortal>
      <UiDialogBackdrop className="overscroll-contain" inset="compact" layer="dialogNested" onClose={onClose}>
        <UiDialogShell size={isImage ? "xl" : "lg"} viewport={isImage ? "visualPreview" : "documentPreview"}>
          <UiDialogHeader
            appearance="plain"
            className="items-center gap-2 px-3 py-1.5"
            closeLabel={t("composer.close_attachment_preview")}
            onClose={onClose}
            title={<span className="block truncate" title={attachment.file.name}>{attachment.file.name}</span>}
          />
          {isImage ? (
            <ComposerImagePreview attachment={attachment} key={attachment.id} onClose={onClose} />
          ) : (
            <ComposerTextPreview attachment={attachment} key={attachment.id} onClose={onClose} />
          )}
        </UiDialogShell>
      </UiDialogBackdrop>
    </UiDialogPortal>
  );
}

function ComposerImagePreview({
  attachment,
  onClose,
}: {
  attachment: ComposerLocalAttachment;
  onClose: () => void;
}) {
  const imageUrl = useComposerLocalFileUrl(attachment.file);
  const [imageFailed, setImageFailed] = useResettableState(false, attachment.file);
  if (imageFailed || !imageUrl) {
    return <AttachmentPreviewState failed={imageFailed} onClose={onClose} />;
  }
  return (
    <div className="flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-(--surface-paper-background) p-3 sm:p-4">
      <img
        alt={attachment.file.name}
        className="max-h-full max-w-full radius-control-md object-contain shadow-(--surface-paper-shadow)"
        draggable={false}
        onError={() => setImageFailed(true)}
        src={imageUrl}
      />
    </div>
  );
}

function ComposerTextPreview({
  attachment,
  onClose,
}: {
  attachment: ComposerLocalAttachment;
  onClose: () => void;
}) {
  const { t } = useI18n();
  const preview = useComposerTextPreview(attachment.file);
  if (preview.status !== "ready") {
    return <AttachmentPreviewState failed={preview.status === "error"} onClose={onClose} />;
  }
  return (
    <div className="flex min-h-0 flex-1 flex-col bg-(--surface-paper-background)">
      {preview.isTruncated ? (
        <p className={cn("shrink-0 border-b border-(--divider-subtle-color) bg-(--surface-panel-subtle-background) px-5 py-2", getUiTypographyClassName({ role: "caption", tone: "muted" }))}>
          {t("composer.text_preview_truncated")}
        </p>
      ) : null}
      <pre
        aria-label={attachment.file.name}
        className={cn("flex-1 whitespace-pre-wrap break-words px-5 py-4 text-(--surface-paper-foreground)", UI_PREVIEW_VIEWPORT_CLASS_NAME, UI_SOURCE_TEXT_CLASS_NAME)}
        role="region"
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex -- Named read-only text needs native keyboard scrolling.
        tabIndex={0}
      >
        {preview.content || t("composer.text_preview_empty")}
      </pre>
    </div>
  );
}

function AttachmentPreviewState({ failed, onClose }: { failed: boolean; onClose: () => void }) {
  const { t } = useI18n();
  return (
    <div className={cn("flex-1 bg-(--surface-paper-background) p-4", UI_PREVIEW_VIEWPORT_CLASS_NAME)}>
      <UiResourceState
        {...(failed ? {
          state: "error" as const,
          impact: t("composer.attachment_preview_failed_impact"),
          primaryAction: { label: t("composer.close_attachment_preview"), onClick: onClose },
        } : { state: "loading" as const })}
        className="mx-auto min-h-0 w-full max-w-md py-5"
        size="sm"
        title={t(failed ? "composer.attachment_preview_failed" : "composer.attachment_preview_loading")}
        urgency="polite"
        variant={failed ? "card" : "plain"}
      />
    </div>
  );
}

function useComposerTextPreview(file: File): TextPreviewState {
  const [preview, setPreview] = useResettableState<TextPreviewState>({
    content: "",
    isTruncated: false,
    status: "loading",
  }, file);

  useEffect(() => {
    let isCurrent = true;
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
  }, [file, setPreview]);

  return preview;
}
