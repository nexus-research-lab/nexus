import {
  File as FileIcon,
  FileText,
  Image as ImageIcon,
  X,
} from "lucide-react";

import { ComposerAttachmentKind } from "./composer-attachments";
import { ComposerLocalAttachment } from "./composer-local-attachment-model";
import {
  COMPOSER_ATTACHMENT_CLASS_NAME,
  COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME,
  COMPOSER_ATTACHMENT_ROW_CLASS_NAME,
} from "./composer-styles";

function get_attachment_kind_label(kind: ComposerAttachmentKind) {
  if (kind === "image") {
    return "图片";
  }
  if (kind === "text") {
    return "文本文件";
  }
  return "工作文件";
}

function get_attachment_icon(kind: ComposerAttachmentKind) {
  if (kind === "image") {
    return ImageIcon;
  }
  if (kind === "text") {
    return FileText;
  }
  return FileIcon;
}

export function ComposerAttachmentList({
  attachments,
  on_remove,
  remove_label,
}: {
  attachments: ComposerLocalAttachment[];
  on_remove: (id: string) => void;
  remove_label: string;
}) {
  if (attachments.length === 0) {
    return null;
  }

  return (
    <div className={COMPOSER_ATTACHMENT_ROW_CLASS_NAME}>
      {attachments.map((attachment) => {
        const AttachmentIcon = get_attachment_icon(attachment.kind);
        return (
          <div
            key={attachment.id}
            className={COMPOSER_ATTACHMENT_CLASS_NAME}
            title={`${get_attachment_kind_label(attachment.kind)}：${attachment.file.name}`}
          >
            <AttachmentIcon size={16} className="text-accent" />
            <span className="max-w-[120px] truncate text-xs text-foreground/70">
              {attachment.file.name}
            </span>
            <button
              aria-label={remove_label}
              className={COMPOSER_ATTACHMENT_REMOVE_CLASS_NAME}
              onClick={() => on_remove(attachment.id)}
            >
              <X size={12} />
            </button>
          </div>
        );
      })}
    </div>
  );
}
