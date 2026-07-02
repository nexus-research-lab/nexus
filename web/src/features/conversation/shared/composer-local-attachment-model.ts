import {
  ComposerAttachmentKind,
  get_attachment_rejection_reason,
  get_composer_attachment_kind,
} from "./composer-attachments";

export interface ComposerLocalAttachment {
  id: string;
  file: File;
  kind: ComposerAttachmentKind;
}

export const MAX_COMPOSER_ATTACHMENTS = 6;
export const PASTED_TEXT_ATTACHMENT_THRESHOLD = 10_000;

const CLIPBOARD_IMAGE_EXTENSION_BY_MIME: Record<string, string> = {
  "image/png": "png",
  "image/jpeg": "jpg",
  "image/webp": "webp",
  "image/gif": "gif",
  "image/bmp": "bmp",
  "image/svg+xml": "svg",
};

function createAttachmentId() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

function buildPastedImageFile(file: File, index: number): File {
  if (!file.type.startsWith("image/")) {
    return file;
  }

  const extension = CLIPBOARD_IMAGE_EXTENSION_BY_MIME[file.type] ?? "png";
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return new File(
    [file],
    `pasted-image-${timestamp}-${index + 1}.${extension}`,
    {
      lastModified: Date.now(),
      type: file.type,
    },
  );
}

export function build_pasted_text_file(text: string): File {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return new File([text], `pasted-text-${timestamp}.txt`, {
    lastModified: Date.now(),
    type: "text/plain",
  });
}

export function get_clipboard_files(clipboardData: DataTransfer): File[] {
  const filesFromItems = Array.from(clipboardData.items)
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
    .map(buildPastedImageFile);

  if (filesFromItems.length > 0) {
    return filesFromItems;
  }

  return Array.from(clipboardData.files).map(buildPastedImageFile);
}

export function build_local_attachment(
  file: File,
  unsupportedMessage: string,
): { attachment: ComposerLocalAttachment | null; rejection_reason: string | null } {
  const rejectionReason = get_attachment_rejection_reason(file);
  if (rejectionReason) {
    return { attachment: null, rejection_reason: rejectionReason };
  }

  const kind = get_composer_attachment_kind(file);
  if (!kind) {
    return { attachment: null, rejection_reason: unsupportedMessage };
  }

  return {
    attachment: {
      id: createAttachmentId(),
      file,
      kind,
    },
    rejection_reason: null,
  };
}
