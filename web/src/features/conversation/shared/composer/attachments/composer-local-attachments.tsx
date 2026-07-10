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
} from "../composer-styles";

function getAttachmentKindLabel(kind: ComposerAttachmentKind) {
  if (kind === "image") {
    return "图片";
  }
  if (kind === "text") {
    return "文本文件";
  }
  return "工作文件";
}

function getAttachmentIcon(kind: ComposerAttachmentKind) {
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
  onRemove: onRemove,
  removeLabel: removeLabel,
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
        const AttachmentIcon = getAttachmentIcon(attachment.kind);
        return (
          <div
            key={attachment.id}
            className={COMPOSER_ATTACHMENT_CLASS_NAME}
            title={`${getAttachmentKindLabel(attachment.kind)}：${attachment.file.name}`}
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
