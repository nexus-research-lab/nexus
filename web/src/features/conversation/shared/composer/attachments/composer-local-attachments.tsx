/**
 * INPUT: Composer 中尚未发送的本地附件与移除动作。
 * OUTPUT: 图片缩略图和共享文件 Chip；预览按 Session 同步重置，移除入口包含文件名。
 * POS: Composer 草稿展示；普通胶囊归 RemovableChip，图片覆盖动作保留独立缩略图几何。
 */
"use client";

import {
  Eye,
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  Maximize2,
  X,
} from "lucide-react";

import { useResettableState } from "@/shared/lib/react/use-resettable-state";
import { UiRemovableChip } from "@/shared/ui/form/removable-chip";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";

import type { ComposerLocalAttachment } from "./composer-local-attachment-model";
import { ComposerAttachmentPreviewDialog } from "./composer-attachment-preview-dialog";
import { useComposerLocalFileUrl } from "./use-composer-local-file-url";
import {
  COMPOSER_ATTACHMENT_ROW_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME,
  COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME,
} from "../composer-styles";

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
  const [previewAttachmentId, setPreviewAttachmentId] = useResettableState<string | null>(null, previewResetKey);
  const previewAttachment = attachments.find(
    (attachment) => attachment.id === previewAttachmentId,
  ) ?? null;

  if (previewAttachmentId !== null && !previewAttachment) {
    setPreviewAttachmentId(null);
  }

  if (attachments.length === 0) {
    return null;
  }

  return (
    <>
      <div className={COMPOSER_ATTACHMENT_ROW_CLASS_NAME}>
        {attachments.map((attachment) => {
          const namedRemoveLabel = `${removeLabel}: ${attachment.file.name}`;
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
                removeLabel={namedRemoveLabel}
              />
            );
          }
          return (
            <ComposerFileAttachment
              attachment={attachment}
              key={attachment.id}
              onPreview={() => setPreviewAttachmentId(attachment.id)}
              onRemove={onRemove}
              previewLabel={t("composer.preview_text", { name: attachment.file.name })}
              removeLabel={namedRemoveLabel}
            />
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
      title={attachment.file.name}
    >
      <UiButton
        aria-label={previewLabel}
        className={COMPOSER_IMAGE_ATTACHMENT_PREVIEW_CLASS_NAME}
        onClick={onPreview}
        size="sm"
        variant="ghost"
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
      </UiButton>
      <UiIconButton
        aria-label={removeLabel}
        className={COMPOSER_IMAGE_ATTACHMENT_REMOVE_CLASS_NAME}
        onClick={() => onRemove(attachment.id)}
        shape="round"
        size="2xs"
        tone="danger"
        tooltip={removeLabel}
        variant="surface"
      >
        <X size={11} />
      </UiIconButton>
    </div>
  );
}

function ComposerFileAttachment({
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
  const isText = attachment.kind === "text";
  const Icon = isText ? FileText : FileIcon;
  const content = <>
    <Icon aria-hidden size={16} className="shrink-0 text-(--icon-default)" />
    <span className="max-w-[120px] truncate">{attachment.file.name}</span>
    {isText ? <Eye aria-hidden className="h-3.5 w-3.5 shrink-0 text-(--icon-muted)" /> : null}
  </>;
  return (
    <UiRemovableChip onRemove={() => onRemove(attachment.id)} removeLabel={removeLabel} size="xs">
      {isText ? (
        <UiButton aria-label={previewLabel} className="min-h-0 min-w-0 gap-1.5 p-0" onClick={onPreview} size="2xs" title={attachment.file.name} variant="ghost">
          {content}
        </UiButton>
      ) : (
        <span className="inline-flex min-w-0 items-center gap-1.5" title={attachment.file.name}>{content}</span>
      )}
    </UiRemovableChip>
  );
}
