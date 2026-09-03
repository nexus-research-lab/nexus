/**
 * INPUT: Composer 中尚未发送的本地附件与移除动作。
 * OUTPUT: 图片真实缩略图、普通文件胶囊及统一的可访问移除入口。
 * POS: Composer 草稿附件的唯一展示层。
 */
"use client";

import { useEffect, useState } from "react";
import {
  Eye,
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Maximize2,
  X,
} from "lucide-react";

import type { MessageAttachmentKind } from "@/types/conversation/message/attachment";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";

import type { ComposerLocalAttachment } from "./composer-local-attachment-model";
import { ComposerAttachmentPreviewDialog } from "./composer-attachment-preview-dialog";
import { useComposerLocalFileUrl } from "./use-composer-local-file-url";
import {
  COMPOSER_ATTACHMENT_CLASS_NAME,
  COMPOSER_ATTACHMENT_PREVIEW_CLASS_NAME,
  COMPOSER_ATTACHMENT_ROW_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME,
} from "../composer-styles";

const ATTACHMENT_PRESENTATION: Record<
  MessageAttachmentKind,
  { icon: typeof FileIcon; label: string }
> = {
  file: { icon: FileIcon, label: "工作文件" },
  image: { icon: ImageIcon, label: "图片" },
  text: { icon: FileText, label: "文本文件" },
};

export function ComposerAttachmentList({
  attachments,
  onRemove,
  previewResetKey,
  removeLabel,
}: {
  attachments: ComposerLocalAttachment[];
  onRemove: (id: string) => void;
  previewResetKey: string;
  removeLabel: string;
}) {
  const { t } = useI18n();
  const [previewAttachmentId, setPreviewAttachmentId] = useState<string | null>(
    null,
  );
  const previewAttachment = attachments.find(
    (attachment) => attachment.id === previewAttachmentId,
  ) ?? null;

  useEffect(() => {
    setPreviewAttachmentId(null);
  }, [previewResetKey]);

  if (attachments.length === 0) {
    return null;
  }

  return (
    <>
      <div className={COMPOSER_ATTACHMENT_ROW_CLASS_NAME}>
        {attachments.map((attachment) => {
          const presentation = ATTACHMENT_PRESENTATION[attachment.kind];
          if (attachment.kind === "image") {
            return (
              <ComposerImageAttachment
                attachment={attachment}
                key={attachment.id}
                onPreview={() => setPreviewAttachmentId(attachment.id)}
                onRemove={onRemove}
                previewLabel={t("composer.preview_image", {
                  name: attachment.file.name,
                })}
                removeLabel={removeLabel}
              />
            );
          }
          if (attachment.kind === "text") {
            return (
              <ComposerTextAttachment
                attachment={attachment}
                key={attachment.id}
                onPreview={() => setPreviewAttachmentId(attachment.id)}
                onRemove={onRemove}
                previewLabel={t("composer.preview_text", {
                  name: attachment.file.name,
                })}
                removeLabel={removeLabel}
              />
            );
          }
          const AttachmentIcon = presentation.icon;
          return (
            <div
              key={attachment.id}
              className={COMPOSER_ATTACHMENT_CLASS_NAME}
              title={`${presentation.label}：${attachment.file.name}`}
            >
              <AttachmentIcon size={16} className="text-(--icon-default)" />
              <span className="max-w-[120px] truncate text-xs text-foreground/70">
                {attachment.file.name}
              </span>
              <UiIconButton
                aria-label={removeLabel}
                className="ml-1 shrink-0 opacity-60 hover:opacity-100"
                onClick={() => onRemove(attachment.id)}
                shape="round"
                size="2xs"
                tone="danger"
                tooltip={removeLabel}
                variant="ghost"
              >
                <X size={12} />
              </UiIconButton>
            </div>
          );
        })}
      </div>
      <ComposerAttachmentPreviewDialog
        attachment={previewAttachment}
        onClose={() => setPreviewAttachmentId(null)}
      />
    </>
  );
}

function ComposerImageAttachment({
  attachment,
  onPreview,
  onRemove,
  previewLabel,
  removeLabel,
}: {
  attachment: ComposerLocalAttachment;
  onPreview: () => void;
  onRemove: (id: string) => void;
  previewLabel: string;
  removeLabel: string;
}) {
  const previewUrl = useComposerLocalFileUrl(attachment.file);

  return (
    <div
      className={COMPOSER_IMAGE_ATTACHMENT_CLASS_NAME}
      title={`图片：${attachment.file.name}`}
    >
      <button
        aria-label={previewLabel}
        className={COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME}
        onClick={onPreview}
        type="button"
      >
        {previewUrl ? (
          <img
            alt={attachment.file.name}
            className="h-full w-full object-cover"
            draggable={false}
            src={previewUrl}
          />
        ) : (
          <ImageIcon
            aria-hidden="true"
            className="text-(--icon-muted)"
            size={18}
          />
        )}
        <span className="pointer-events-none absolute inset-0 flex items-end justify-start bg-black/0 p-1.5 transition-colors group-hover/preview:bg-black/10 group-focus-visible/preview:bg-black/10">
          <Maximize2 className="h-3.5 w-3.5 text-white opacity-0 drop-shadow-sm transition-opacity group-hover/preview:opacity-100 group-focus-visible/preview:opacity-100" />
        </span>
      </button>
      <button
        aria-label={removeLabel}
        className={COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME}
        onClick={() => onRemove(attachment.id)}
        type="button"
      >
        <X size={11} />
      </button>
    </div>
  );
}

function ComposerTextAttachment({
  attachment,
  onPreview,
  onRemove,
  previewLabel,
  removeLabel,
}: {
  attachment: ComposerLocalAttachment;
  onPreview: () => void;
  onRemove: (id: string) => void;
  previewLabel: string;
  removeLabel: string;
}) {
  return (
    <div
      className={COMPOSER_ATTACHMENT_CLASS_NAME}
      title={`文本文件：${attachment.file.name}`}
    >
      <button
        aria-label={previewLabel}
        className={COMPOSER_ATTACHMENT_PREVIEW_CLASS_NAME}
        onClick={onPreview}
        type="button"
      >
        <FileText size={16} className="shrink-0 text-(--icon-default)" />
        <span className="max-w-[120px] truncate text-xs text-foreground/70">
          {attachment.file.name}
        </span>
        <Eye className="h-3.5 w-3.5 shrink-0 text-(--icon-muted) opacity-55" />
      </button>
      <UiIconButton
        aria-label={removeLabel}
        className="ml-1 shrink-0 opacity-60 hover:opacity-100"
        onClick={() => onRemove(attachment.id)}
        shape="round"
        size="2xs"
        tone="danger"
        tooltip={removeLabel}
        variant="ghost"
      >
        <X size={12} />
      </UiIconButton>
    </div>
  );
}
