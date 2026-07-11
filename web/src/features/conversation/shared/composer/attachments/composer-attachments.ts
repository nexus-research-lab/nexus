import { uploadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { uploadRoomConversationAttachmentApi } from "@/lib/api/conversation/room-command-api";
import type { MessageAttachment } from "@/types/conversation/message";

export type ComposerAttachmentKind = "text" | "image" | "file";

export interface PreparedComposerAttachment extends MessageAttachment {}

const SUPPORTED_TEXT_ATTACHMENT_EXTENSIONS = new Set([
  "txt",
  "md",
  "markdown",
  "json",
  "jsonl",
  "yaml",
  "yml",
  "csv",
  "ts",
  "tsx",
  "js",
  "jsx",
  "mjs",
  "cjs",
  "py",
  "java",
  "go",
  "rs",
  "rb",
  "php",
  "sh",
  "bash",
  "zsh",
  "sql",
  "xml",
  "html",
  "css",
  "scss",
  "less",
  "log",
  "ini",
  "toml",
  "env",
  "conf",
  "svg",
  "rst",
  "adoc",
]);

const SUPPORTED_IMAGE_ATTACHMENT_EXTENSIONS = new Set([
  "png",
  "jpg",
  "jpeg",
  "webp",
  "gif",
  "bmp",
  "svg",
]);

const SUPPORTED_WORK_FILE_EXTENSIONS = new Set([
  "pdf",
  "doc",
  "docx",
  "xls",
  "xlsx",
  "ppt",
  "pptx",
  "rtf",
  "odt",
  "ods",
  "odp",
]);

const SUPPORTED_TEXT_MIME_PREFIXES = [
  "text/",
];

const SUPPORTED_IMAGE_MIME_PREFIXES = [
  "image/",
];

const SUPPORTED_TEXT_MIME_TYPES = new Set([
  "application/json",
  "application/ld+json",
  "application/xml",
  "application/x-yaml",
  "application/yaml",
  "image/svg+xml",
]);

const SUPPORTED_WORK_FILE_MIME_TYPES = new Set([
  "application/pdf",
  "application/msword",
  "application/vnd.ms-excel",
  "application/vnd.ms-powerpoint",
  "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
  "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
  "application/vnd.openxmlformats-officedocument.presentationml.presentation",
  "application/rtf",
  "application/vnd.oasis.opendocument.text",
  "application/vnd.oasis.opendocument.spreadsheet",
  "application/vnd.oasis.opendocument.presentation",
]);

const MAX_ATTACHMENT_SIZE_BYTES = 20 * 1024 * 1024;
const ATTACHMENT_DIRECTORY = "tmp/attachments";

export const COMPOSER_ATTACHMENT_ACCEPT =
  [
    ".png",
    ".jpg",
    ".jpeg",
    ".webp",
    ".gif",
    ".bmp",
    ".svg",
    ".pdf",
    ".doc",
    ".docx",
    ".xls",
    ".xlsx",
    ".ppt",
    ".pptx",
    ".rtf",
    ".odt",
    ".ods",
    ".odp",
    ".txt",
    ".md",
    ".markdown",
    ".json",
    ".jsonl",
    ".yaml",
    ".yml",
    ".csv",
    ".ts",
    ".tsx",
    ".js",
    ".jsx",
    ".mjs",
    ".cjs",
    ".py",
    ".java",
    ".go",
    ".rs",
    ".rb",
    ".php",
    ".sh",
    ".bash",
    ".zsh",
    ".sql",
    ".xml",
    ".html",
    ".css",
    ".scss",
    ".less",
    ".log",
    ".ini",
    ".toml",
    ".env",
    ".conf",
    ".rst",
    ".adoc",
  ].join(",");

function getFileExtension(fileName: string): string {
  const normalizedName = fileName.trim().toLowerCase();
  const dotIndex = normalizedName.lastIndexOf(".");
  if (dotIndex < 0 || dotIndex === normalizedName.length - 1) {
    return "";
  }
  return normalizedName.slice(dotIndex + 1);
}

function sanitizeAttachmentName(fileName: string): string {
  const trimmedName = fileName.trim() || "attachment.txt";
  const sanitizedName = trimmedName
    .replace(/[^\w.\-\u4e00-\u9fa5]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return sanitizedName || "attachment.txt";
}

function isSupportedImageAttachment(file: File): boolean {
  const extension = getFileExtension(file.name);
  if (SUPPORTED_IMAGE_ATTACHMENT_EXTENSIONS.has(extension)) {
    return true;
  }

  return SUPPORTED_IMAGE_MIME_PREFIXES.some((prefix) => file.type.startsWith(prefix));
}

function isSupportedTextAttachment(file: File): boolean {
  const extension = getFileExtension(file.name);
  if (SUPPORTED_TEXT_ATTACHMENT_EXTENSIONS.has(extension)) {
    return true;
  }

  if (SUPPORTED_TEXT_MIME_TYPES.has(file.type)) {
    return true;
  }

  return SUPPORTED_TEXT_MIME_PREFIXES.some((prefix) => file.type.startsWith(prefix));
}

function isSupportedWorkFileAttachment(file: File): boolean {
  const extension = getFileExtension(file.name);
  if (SUPPORTED_WORK_FILE_EXTENSIONS.has(extension)) {
    return true;
  }

  return SUPPORTED_WORK_FILE_MIME_TYPES.has(file.type);
}

export function getComposerAttachmentKind(file: File): ComposerAttachmentKind | null {
  if (isSupportedImageAttachment(file)) {
    return "image";
  }
  if (isSupportedTextAttachment(file)) {
    return "text";
  }
  if (isSupportedWorkFileAttachment(file)) {
    return "file";
  }
  return null;
}

export function getAttachmentRejectionReason(file: File): string | null {
  if (!getComposerAttachmentKind(file)) {
    return `暂不支持该附件格式：${file.name}`;
  }

  if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
    return `附件过大，请控制在 20MB 内：${file.name}`;
  }

  return null;
}

function buildAttachmentDirectory(batchId: string, index: number): string {
  return `${ATTACHMENT_DIRECTORY}/${batchId}-${index + 1}/`;
}

function buildRoomAttachmentDirectory(batchId: string, index: number): string {
  return `attachments/${batchId}-${index + 1}/`;
}

function buildUploadFile(file: File): File {
  const safeName = sanitizeAttachmentName(file.name);
  if (safeName === file.name) {
    return file;
  }

  return new File([file], safeName, {
    lastModified: file.lastModified,
    type: file.type,
  });
}

export async function prepareWorkspaceAttachments(
  agentId: string,
  files: File[],
): Promise<PreparedComposerAttachment[]> {
  const nextAttachments: PreparedComposerAttachment[] = [];
  const batchId = new Date().toISOString().replace(/[:.]/g, "-");

  for (const [index, file] of files.entries()) {
    const rejectionReason = getAttachmentRejectionReason(file);
    if (rejectionReason) {
      throw new Error(rejectionReason);
    }

    const kind = getComposerAttachmentKind(file);
    if (!kind) {
      throw new Error(`暂不支持该附件格式：${file.name}`);
    }

    const uploadFile = buildUploadFile(file);
    const uploadedFile = await uploadWorkspaceFileApi(
      agentId,
      uploadFile,
      buildAttachmentDirectory(batchId, index),
    );
    const preparedAttachment: PreparedComposerAttachment = {
      file_name: file.name || uploadedFile.name,
      workspace_path: uploadedFile.path,
      workspace_agent_id: agentId,
      scope: "agentWorkspace",
      kind,
      mime_type: file.type || null,
      size: uploadedFile.size,
    };

    nextAttachments.push(preparedAttachment);
  }

  return nextAttachments;
}

export async function prepareRoomConversationAttachments(
  roomId: string,
  conversationId: string,
  files: File[],
): Promise<PreparedComposerAttachment[]> {
  const nextAttachments: PreparedComposerAttachment[] = [];
  const batchId = new Date().toISOString().replace(/[:.]/g, "-");

  for (const [index, file] of files.entries()) {
    const rejectionReason = getAttachmentRejectionReason(file);
    if (rejectionReason) {
      throw new Error(rejectionReason);
    }

    const kind = getComposerAttachmentKind(file);
    if (!kind) {
      throw new Error(`暂不支持该附件格式：${file.name}`);
    }

    const uploadFile = buildUploadFile(file);
    const uploadedFile = await uploadRoomConversationAttachmentApi(
      roomId,
      conversationId,
      uploadFile,
      buildRoomAttachmentDirectory(batchId, index),
    );
    const preparedAttachment: PreparedComposerAttachment = {
      file_name: file.name || uploadedFile.name,
      workspace_path: uploadedFile.path,
      room_id: roomId,
      conversation_id: conversationId,
      scope: "roomConversation",
      kind,
      mime_type: file.type || null,
      size: uploadedFile.size,
    };

    nextAttachments.push(preparedAttachment);
  }

  return nextAttachments;
}
