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

function create_attachment_id() {
  return `${Date.now()}-${Math.random().toString(36).slice(2, 11)}`;
}

function build_pasted_image_file(file: File, index: number): File {
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

export function get_clipboard_files(clipboard_data: DataTransfer): File[] {
  const files_from_items = Array.from(clipboard_data.items)
    .filter((item) => item.kind === "file")
    .map((item) => item.getAsFile())
    .filter((file): file is File => Boolean(file))
    .map(build_pasted_image_file);

  if (files_from_items.length > 0) {
    return files_from_items;
  }

  return Array.from(clipboard_data.files).map(build_pasted_image_file);
}

export function build_local_attachment(
  file: File,
  unsupported_message: string,
): { attachment: ComposerLocalAttachment | null; rejection_reason: string | null } {
  const rejection_reason = get_attachment_rejection_reason(file);
  if (rejection_reason) {
    return { attachment: null, rejection_reason };
  }

  const kind = get_composer_attachment_kind(file);
  if (!kind) {
    return { attachment: null, rejection_reason: unsupported_message };
  }

  return {
    attachment: {
      id: create_attachment_id(),
      file,
      kind,
    },
    rejection_reason: null,
  };
}
