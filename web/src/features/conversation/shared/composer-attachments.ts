import { upload_workspace_file_api } from "@/lib/api/agent-manage-api";
import { upload_room_conversation_attachment_api } from "@/lib/api/room-api";
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

function get_file_extension(file_name: string): string {
  const normalized_name = file_name.trim().toLowerCase();
  const dot_index = normalized_name.lastIndexOf(".");
  if (dot_index < 0 || dot_index === normalized_name.length - 1) {
    return "";
  }
  return normalized_name.slice(dot_index + 1);
}

function sanitize_attachment_name(file_name: string): string {
  const trimmed_name = file_name.trim() || "attachment.txt";
  const sanitized_name = trimmed_name
    .replace(/[^\w.\-\u4e00-\u9fa5]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return sanitized_name || "attachment.txt";
}

function is_supported_image_attachment(file: File): boolean {
  const extension = get_file_extension(file.name);
  if (SUPPORTED_IMAGE_ATTACHMENT_EXTENSIONS.has(extension)) {
    return true;
  }

  return SUPPORTED_IMAGE_MIME_PREFIXES.some((prefix) => file.type.startsWith(prefix));
}

export function is_supported_text_attachment(file: File): boolean {
  const extension = get_file_extension(file.name);
  if (SUPPORTED_TEXT_ATTACHMENT_EXTENSIONS.has(extension)) {
    return true;
  }

  if (SUPPORTED_TEXT_MIME_TYPES.has(file.type)) {
    return true;
  }

  return SUPPORTED_TEXT_MIME_PREFIXES.some((prefix) => file.type.startsWith(prefix));
}

function is_supported_work_file_attachment(file: File): boolean {
  const extension = get_file_extension(file.name);
  if (SUPPORTED_WORK_FILE_EXTENSIONS.has(extension)) {
    return true;
  }

  return SUPPORTED_WORK_FILE_MIME_TYPES.has(file.type);
}

export function get_composer_attachment_kind(file: File): ComposerAttachmentKind | null {
  if (is_supported_image_attachment(file)) {
    return "image";
  }
  if (is_supported_text_attachment(file)) {
    return "text";
  }
  if (is_supported_work_file_attachment(file)) {
    return "file";
  }
  return null;
}

export function get_attachment_rejection_reason(file: File): string | null {
  if (!get_composer_attachment_kind(file)) {
    return `暂不支持该附件格式：${file.name}`;
  }

  if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
    return `附件过大，请控制在 20MB 内：${file.name}`;
  }

  return null;
}

function build_attachment_directory(batch_id: string, index: number): string {
  return `${ATTACHMENT_DIRECTORY}/${batch_id}-${index + 1}/`;
}

function build_room_attachment_directory(batch_id: string, index: number): string {
  return `attachments/${batch_id}-${index + 1}/`;
}

function build_upload_file(file: File): File {
  const safe_name = sanitize_attachment_name(file.name);
  if (safe_name === file.name) {
    return file;
  }

  return new File([file], safe_name, {
    lastModified: file.lastModified,
    type: file.type,
  });
}

export async function prepare_workspace_attachments(
  agent_id: string,
  files: File[],
): Promise<PreparedComposerAttachment[]> {
  const next_attachments: PreparedComposerAttachment[] = [];
  const batch_id = new Date().toISOString().replace(/[:.]/g, "-");

  for (const [index, file] of files.entries()) {
    const rejection_reason = get_attachment_rejection_reason(file);
    if (rejection_reason) {
      throw new Error(rejection_reason);
    }

    const kind = get_composer_attachment_kind(file);
    if (!kind) {
      throw new Error(`暂不支持该附件格式：${file.name}`);
    }

    const upload_file = build_upload_file(file);
    const uploaded_file = await upload_workspace_file_api(
      agent_id,
      upload_file,
      build_attachment_directory(batch_id, index),
    );
    const prepared_attachment: PreparedComposerAttachment = {
      file_name: file.name || uploaded_file.name,
      workspace_path: uploaded_file.path,
      workspace_agent_id: agent_id,
      scope: "agent_workspace",
      kind,
      mime_type: file.type || null,
      size: uploaded_file.size,
    };

    next_attachments.push(prepared_attachment);
  }

  return next_attachments;
}

export async function prepare_room_conversation_attachments(
  room_id: string,
  conversation_id: string,
  files: File[],
): Promise<PreparedComposerAttachment[]> {
  const next_attachments: PreparedComposerAttachment[] = [];
  const batch_id = new Date().toISOString().replace(/[:.]/g, "-");

  for (const [index, file] of files.entries()) {
    const rejection_reason = get_attachment_rejection_reason(file);
    if (rejection_reason) {
      throw new Error(rejection_reason);
    }

    const kind = get_composer_attachment_kind(file);
    if (!kind) {
      throw new Error(`暂不支持该附件格式：${file.name}`);
    }

    const upload_file = build_upload_file(file);
    const uploaded_file = await upload_room_conversation_attachment_api(
      room_id,
      conversation_id,
      upload_file,
      build_room_attachment_directory(batch_id, index),
    );
    const prepared_attachment: PreparedComposerAttachment = {
      file_name: file.name || uploaded_file.name,
      workspace_path: uploaded_file.path,
      room_id,
      conversation_id,
      scope: "room_conversation",
      kind,
      mime_type: file.type || null,
      size: uploaded_file.size,
    };

    next_attachments.push(prepared_attachment);
  }

  return next_attachments;
}
