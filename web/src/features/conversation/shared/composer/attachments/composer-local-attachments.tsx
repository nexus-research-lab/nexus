import {
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  X,
} from "lucide-react";

import type { MessageAttachmentKind } from "@/types/conversation/message/attachment";

import type { ComposerLocalAttachment } from "./composer-local-attachment-model";
import {
  COMPOSER_ATTACHMENT_CLASS_NAME,
  COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME,
  COMPOSER_ATTACHMENT_ROW_CLASS_NAME,
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
  removeLabel,
}: {
  attachments: ComposerLocalAttachment[];
  onRemove: (id: string) => void;
  removeLabel: string;
}) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className={COMPOSER_ATTACHMENT_ROW_CLASS_NAME}>
      {attachments.map((attachment) => {
        const presentation = ATTACHMENT_PRESENTATION[attachment.kind];
        const AttachmentIcon = presentation.icon;
        return (
          <div
            key={attachment.id}
            className={COMPOSER_ATTACHMENT_CLASS_NAME}
            title={`${presentation.label}：${attachment.file.name}`}
          >
            <AttachmentIcon size={16} className="text-accent" />
            <span className="max-w-[120px] truncate text-xs text-foreground/70">
              {attachment.file.name}
            </span>
            <button
              aria-label={removeLabel}
              className={COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME}
              onClick={() => onRemove(attachment.id)}
              type="button"
            >
              <X size={12} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
