import type { MessageAttachmentKind } from "@/types/conversation/message/attachment";

import {
  type ComposerAttachmentRejection,
  inspectComposerAttachment,
} from "./composer-attachments";

export interface ComposerLocalAttachment {
  id: string;
  file: File;
  kind: MessageAttachmentKind;
}

export interface ComposerLocalAttachmentBatch {
  attachments: ComposerLocalAttachment[];
  rejections: ComposerAttachmentRejection[];
}

export type ComposerPasteActionKind = "append_files" | "append_text" | "native" | "reject_goal";

export interface ComposerPasteAction {
  files: File[];
  kind: ComposerPasteActionKind;
  text: string;
}

const MAX_COMPOSER_ATTACHMENTS = 6;
const PASTED_TEXT_ATTACHMENT_THRESHOLD = 10_000;

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

export function buildPastedTextFile(text: string): File {
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return new File([text], `pasted-text-${timestamp}.txt`, {
    lastModified: Date.now(),
    type: "text/plain",
  });
}

function getClipboardFiles(clipboardData: DataTransfer): File[] {
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

function buildLocalAttachment(
  file: File,
): {
  attachment: ComposerLocalAttachment | null;
  rejection: ComposerAttachmentRejection | null;
} {
  const inspection = inspectComposerAttachment(file);
  if (!inspection.accepted) {
    return { attachment: null, rejection: inspection };
  }

  return {
    attachment: {
      id: createAttachmentId(),
      file,
      kind: inspection.kind,
    },
    rejection: null,
  };
}

export function buildLocalAttachmentBatch(
  files: File[],
): ComposerLocalAttachmentBatch {
  const results = files.map(buildLocalAttachment);
  return {
    attachments: results.flatMap((result) => (
      result.attachment ? [result.attachment] : []
    )),
    rejections: results.flatMap((result) => (
      result.rejection ? [result.rejection] : []
    )),
  };
}

export function appendLocalAttachments(
  current: ComposerLocalAttachment[],
  additions: ComposerLocalAttachment[],
): ComposerLocalAttachment[] {
  if (additions.length === 0) {
    return current;
  }
  return [...current, ...additions].slice(0, MAX_COMPOSER_ATTACHMENTS);
}

export function projectComposerPasteAction(
  clipboardData: DataTransfer,
  isGoalMode: boolean,
): ComposerPasteAction {
  const files = getClipboardFiles(clipboardData);
  const text = clipboardData.getData("text/plain");
  const candidates: Array<{
    active: boolean;
    kind: ComposerPasteActionKind;
  }> = [
    {
      active: [files.length > 0, isGoalMode].every(Boolean),
      kind: "reject_goal",
    },
    { active: files.length > 0, kind: "append_files" },
    {
      active: [
        !isGoalMode,
        text.length > PASTED_TEXT_ATTACHMENT_THRESHOLD,
      ].every(Boolean),
      kind: "append_text",
    },
  ];
  return {
    files,
    kind: candidates.find((candidate) => candidate.active)?.kind ?? "native",
    text,
  };
}
