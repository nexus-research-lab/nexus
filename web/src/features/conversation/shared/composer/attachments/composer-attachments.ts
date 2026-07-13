import { uploadWorkspaceFileApi } from "@/lib/api/agent/agent-api";
import { uploadRoomConversationAttachmentApi } from "@/lib/api/conversation/room-command-api";
import type {
  MessageAttachment,
  MessageAttachmentKind,
} from "@/types/conversation/message/attachment";

interface AttachmentRule {
  extensions: readonly string[];
  kind: MessageAttachmentKind;
  mimePrefixes?: readonly string[];
  mimeTypes?: readonly string[];
}

export type ComposerAttachmentRejectionCode = "too_large" | "unsupported_format";

export interface ComposerAttachmentRejection {
  code: ComposerAttachmentRejectionCode;
  fileName: string;
}

export type ComposerAttachmentInspection =
  | { accepted: true; kind: MessageAttachmentKind }
  | ({ accepted: false } & ComposerAttachmentRejection);

export class ComposerAttachmentRejectedError extends Error {
  readonly rejection: ComposerAttachmentRejection;

  constructor(rejection: ComposerAttachmentRejection) {
    super(rejection.code);
    this.name = "ComposerAttachmentRejectedError";
    this.rejection = rejection;
  }
}

interface AttachmentUploadResult {
  name: string;
  path: string;
  size: number;
}

type AttachmentScopeFields =
  | {
    scope: "agentWorkspace";
    workspace_agent_id: string;
  }
  | {
    conversation_id: string;
    room_id: string;
    scope: "roomConversation";
  };

interface AttachmentUploadDestination {
  directoryRoot: string;
  scopeFields: AttachmentScopeFields;
  upload: (file: File, path: string) => Promise<AttachmentUploadResult>;
}

interface AttachmentUploadInput {
  displayName: string;
  file: File;
  kind: MessageAttachmentKind;
  mimeType: string | null;
}

const ATTACHMENT_RULES: readonly AttachmentRule[] = [
  {
    extensions: ["png", "jpg", "jpeg", "webp", "gif", "bmp", "svg"],
    kind: "image",
    mimePrefixes: ["image/"],
  },
  {
    extensions: [
      "txt", "md", "markdown", "json", "jsonl", "yaml", "yml", "csv",
      "ts", "tsx", "js", "jsx", "mjs", "cjs", "py", "java", "go", "rs",
      "rb", "php", "sh", "bash", "zsh", "sql", "xml", "html", "css",
      "scss", "less", "log", "ini", "toml", "env", "conf", "svg", "rst",
      "adoc",
    ],
    kind: "text",
    mimePrefixes: ["text/"],
    mimeTypes: [
      "application/json",
      "application/ld+json",
      "application/xml",
      "application/x-yaml",
      "application/yaml",
      "image/svg+xml",
    ],
  },
  {
    extensions: [
      "pdf", "doc", "docx", "xls", "xlsx", "ppt", "pptx", "rtf", "odt",
      "ods", "odp",
    ],
    kind: "file",
    mimeTypes: [
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
    ],
  },
];

const MAX_ATTACHMENT_SIZE_BYTES = 20 * 1024 * 1024;
const WORKSPACE_ATTACHMENT_DIRECTORY = "tmp/attachments";
const ROOM_ATTACHMENT_DIRECTORY = "attachments";

export const COMPOSER_ATTACHMENT_ACCEPT = [
  ...new Set(ATTACHMENT_RULES.flatMap((rule) => rule.extensions)),
]
  .map((extension) => `.${extension}`)
  .join(",");

function getFileExtension(fileName: string): string {
  const normalizedName = fileName.trim().toLowerCase();
  const dotIndex = normalizedName.lastIndexOf(".");
  return dotIndex < 0 || dotIndex === normalizedName.length - 1
    ? ""
    : normalizedName.slice(dotIndex + 1);
}

function matchesAttachmentRule(
  rule: AttachmentRule,
  extension: string,
  mimeType: string,
): boolean {
  return rule.extensions.includes(extension)
    || rule.mimeTypes?.includes(mimeType) === true
    || rule.mimePrefixes?.some((prefix) => mimeType.startsWith(prefix)) === true;
}

export function inspectComposerAttachment(file: File): ComposerAttachmentInspection {
  const extension = getFileExtension(file.name);
  const rule = ATTACHMENT_RULES.find((candidate) => (
    matchesAttachmentRule(candidate, extension, file.type)
  ));
  if (!rule) {
    return {
      accepted: false,
      code: "unsupported_format",
      fileName: file.name,
    };
  }
  if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
    return {
      accepted: false,
      code: "too_large",
      fileName: file.name,
    };
  }
  return { accepted: true, kind: rule.kind };
}

function sanitizeAttachmentName(fileName: string): string {
  const trimmedName = fileName.trim() || "attachment.txt";
  const sanitizedName = trimmedName
    .replace(/[^\w.\-\u4e00-\u9fa5]+/g, "-")
    .replace(/-+/g, "-")
    .replace(/^-|-$/g, "");
  return sanitizedName || "attachment.txt";
}

function buildUploadFile(file: File): File {
  const safeName = sanitizeAttachmentName(file.name);
  return safeName === file.name
    ? file
    : new File([file], safeName, {
      lastModified: file.lastModified,
      type: file.type,
    });
}

function buildUploadInput(file: File): AttachmentUploadInput {
  const inspection = inspectComposerAttachment(file);
  if (!inspection.accepted) {
    throw new ComposerAttachmentRejectedError(inspection);
  }
  return {
    displayName: file.name,
    file: buildUploadFile(file),
    kind: inspection.kind,
    mimeType: file.type || null,
  };
}

function buildAttachmentDirectory(
  root: string,
  batchId: string,
  index: number,
): string {
  return `${root}/${batchId}-${index + 1}/`;
}

async function prepareComposerAttachments(
  files: File[],
  destination: AttachmentUploadDestination,
): Promise<MessageAttachment[]> {
  // 先完成整批检查，避免后续文件无效时留下半批已上传资源。
  const inputs = files.map(buildUploadInput);
  const batchId = new Date().toISOString().replace(/[:.]/g, "-");
  const attachments: MessageAttachment[] = [];

  for (const [index, input] of inputs.entries()) {
    const uploaded = await destination.upload(
      input.file,
      buildAttachmentDirectory(destination.directoryRoot, batchId, index),
    );
    attachments.push({
      file_name: input.displayName || uploaded.name,
      workspace_path: uploaded.path,
      kind: input.kind,
      mime_type: input.mimeType,
      size: uploaded.size,
      ...destination.scopeFields,
    });
  }

  return attachments;
}

export function prepareWorkspaceAttachments(
  agentId: string,
  files: File[],
): Promise<MessageAttachment[]> {
  return prepareComposerAttachments(files, {
    directoryRoot: WORKSPACE_ATTACHMENT_DIRECTORY,
    scopeFields: { scope: "agentWorkspace", workspace_agent_id: agentId },
    upload: (file, path) => uploadWorkspaceFileApi(agentId, file, path),
  });
}

export function prepareRoomConversationAttachments(
  roomId: string,
  conversationId: string,
  files: File[],
): Promise<MessageAttachment[]> {
  return prepareComposerAttachments(files, {
    directoryRoot: ROOM_ATTACHMENT_DIRECTORY,
    scopeFields: {
      conversation_id: conversationId,
      room_id: roomId,
      scope: "roomConversation",
    },
    upload: (file, path) => uploadRoomConversationAttachmentApi(
      roomId,
      conversationId,
      file,
      path,
    ),
  });
}
