import type { ArtifactItem, ArtifactKind } from "@/types/conversation/artifact";
import type {
  AssistantMessage,
  ContentBlock,
  ImageContent,
  Message,
  TextContent,
  WorkspaceFileArtifactContent,
} from "@/types/conversation/message";

import {
  artifact_kind_from_path,
  file_name_from_artifact_path,
  is_workspace_artifact_reference,
  normalize_artifact_path,
} from "./artifact-utils";

const MARKDOWN_IMAGE_PATTERN = /!\[(?<alt>[^\]]*)]\((?<src>[^)\s]+)(?:\s+"[^"]*")?\)/g;
const MARKDOWN_SAVED_PATH_PATTERN = /(?:已保存到|保存到|写入到|生成到|路径|created at|saved to|written to)\s*[：:]?\s*[`"']?(?<path>\/[^\s`"'，。；！？]+\/\.nexus\/workspace\/[^/\s`"'，。；！？]+\/[^\s`"'，。；！？]+\.[A-Za-z0-9]{1,10}|[A-Za-z0-9_.-][A-Za-z0-9_./-]*\.[A-Za-z0-9]{1,10})[`"']?/gi;
const CODE_FENCE_PATTERN = /```(?<language>[A-Za-z0-9_-]+)?[^\n]*\n(?<content>[\s\S]*?)```/g;

interface ArtifactDraft extends Omit<ArtifactItem, "id" | "created_at"> {
  id?: string;
  created_at?: number;
}

interface ArtifactContext {
  message_id: string;
  round_id: string;
  timestamp: number;
  workspace_agent_id: string | null;
  sequence: number;
}

function artifact_key(item: ArtifactItem): string {
  if (item.path && item.workspace_agent_id) {
    return `workspace:${item.workspace_agent_id}:${item.path}`;
  }
  if (item.path) {
    return `workspace:${item.path}`;
  }
  return item.id;
}

function add_artifact(
  artifacts: Map<string, ArtifactItem>,
  draft: ArtifactDraft,
  context: ArtifactContext,
): ArtifactContext {
  const created_at = draft.created_at ?? context.timestamp;
  const id = draft.id || `${context.message_id}:artifact:${context.sequence}`;
  const item: ArtifactItem = {
    ...draft,
    id,
    title: draft.title || draft.path || draft.language || "产物",
    workspace_agent_id: draft.workspace_agent_id ?? context.workspace_agent_id,
    message_id: draft.message_id ?? context.message_id,
    round_id: draft.round_id ?? context.round_id,
    created_at,
  };
  const key = artifact_key(item);
  const existing = artifacts.get(key);
  if (!existing || item.source === "workspace_file_artifact") {
    artifacts.set(key, item);
  }
  return { ...context, sequence: context.sequence + 1 };
}

function text_artifact_kind(language: string): ArtifactKind | null {
  const normalized = language.trim().toLowerCase();
  if (normalized === "mermaid" || normalized === "mmd") {
    return "mermaid";
  }
  if (normalized === "html" || normalized === "htm") {
    return "html";
  }
  if (normalized === "svg") {
    return "svg";
  }
  if (normalized === "markdown" || normalized === "md") {
    return "markdown";
  }
  return null;
}

function collect_markdown_artifacts(
  artifacts: Map<string, ArtifactItem>,
  text: string,
  context: ArtifactContext,
): ArtifactContext {
  let next_context = context;

  for (const match of text.matchAll(MARKDOWN_IMAGE_PATTERN)) {
    const raw_src = match.groups?.src?.trim() ?? "";
    if (!raw_src) {
      continue;
    }
    const alt = match.groups?.alt?.trim();
    if (/^(https?:|data:|blob:)/i.test(raw_src)) {
      next_context = add_artifact(artifacts, {
        kind: "image",
        source: "markdown_image",
        title: alt || "图片",
        url: raw_src,
      }, next_context);
      continue;
    }
    if (!is_workspace_artifact_reference(raw_src)) {
      continue;
    }
    const path = normalize_artifact_path(raw_src);
    next_context = add_artifact(artifacts, {
      kind: artifact_kind_from_path(path),
      source: "markdown_image",
      title: alt || file_name_from_artifact_path(path),
      path,
      display_path: path,
    }, next_context);
  }

  for (const match of text.matchAll(MARKDOWN_SAVED_PATH_PATTERN)) {
    const raw_path = match.groups?.path?.trim() ?? "";
    if (!raw_path || !is_workspace_artifact_reference(raw_path)) {
      continue;
    }
    const path = normalize_artifact_path(raw_path);
    next_context = add_artifact(artifacts, {
      kind: artifact_kind_from_path(path),
      source: "markdown_path",
      title: file_name_from_artifact_path(path),
      path,
      display_path: path,
    }, next_context);
  }

  for (const match of text.matchAll(CODE_FENCE_PATTERN)) {
    const language = match.groups?.language?.trim() ?? "";
    const kind = text_artifact_kind(language);
    const content = match.groups?.content?.trim() ?? "";
    if (!kind || !content) {
      continue;
    }
    next_context = add_artifact(artifacts, {
      kind,
      source: "code_fence",
      title: kind === "mermaid" ? "Mermaid 图表" : `${language.toUpperCase()} 产物`,
      content,
      language,
    }, next_context);
  }

  return next_context;
}

function first_non_empty(...values: Array<string | null | undefined>): string {
  for (const value of values) {
    const trimmed = value?.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return "";
}

function collect_image_block_artifact(
  artifacts: Map<string, ArtifactItem>,
  block: ImageContent,
  context: ArtifactContext,
): ArtifactContext {
  const source = block.source;
  const raw_path = first_non_empty(block.path, block.url, block.uri, source?.path, source?.url, source?.uri);
  if (!raw_path) {
    return context;
  }
  if (/^(https?:|data:|blob:)/i.test(raw_path)) {
    return add_artifact(artifacts, {
      kind: "image",
      source: "image_block",
      title: block.alt?.trim() || "图片",
      url: raw_path,
      mime_type: first_non_empty(block.mime_type, source?.mime_type, source?.media_type),
    }, context);
  }
  if (!is_workspace_artifact_reference(raw_path)) {
    return context;
  }
  const path = normalize_artifact_path(raw_path);
  return add_artifact(artifacts, {
    kind: artifact_kind_from_path(path, first_non_empty(block.mime_type, source?.mime_type, source?.media_type)),
    source: "image_block",
    title: block.alt?.trim() || file_name_from_artifact_path(path),
    path,
    display_path: path,
    mime_type: first_non_empty(block.mime_type, source?.mime_type, source?.media_type),
  }, context);
}

function collect_workspace_file_artifact(
  artifacts: Map<string, ArtifactItem>,
  block: WorkspaceFileArtifactContent,
  context: ArtifactContext,
): ArtifactContext {
  const path = normalize_artifact_path(block.path);
  if (!path) {
    return context;
  }
  const kind = (block.artifact_kind?.trim() || artifact_kind_from_path(path, block.mime_type)) as ArtifactKind;
  return add_artifact(artifacts, {
    id: block.id,
    kind,
    source: "workspace_file_artifact",
    title: block.title?.trim() || block.label?.trim() || file_name_from_artifact_path(path),
    path,
    display_path: block.display_path ?? path,
    workspace_agent_id: block.workspace_agent_id ?? context.workspace_agent_id,
    mime_type: block.mime_type ?? null,
  }, context);
}

function collect_content_block_artifacts(
  artifacts: Map<string, ArtifactItem>,
  block: ContentBlock,
  context: ArtifactContext,
): ArtifactContext {
  if (block.type === "text") {
    return collect_markdown_artifacts(artifacts, (block as TextContent).text, context);
  }
  if (block.type === "workspace_file_artifact") {
    return collect_workspace_file_artifact(artifacts, block, context);
  }
  if (block.type === "image") {
    return collect_image_block_artifact(artifacts, block, context);
  }
  return context;
}

function message_artifact_context(message: AssistantMessage): ArtifactContext {
  return {
    message_id: message.message_id,
    round_id: message.round_id,
    timestamp: message.timestamp,
    workspace_agent_id: message.agent_id || null,
    sequence: 0,
  };
}

export function collect_conversation_artifacts(messages: Message[]): ArtifactItem[] {
  const artifacts = new Map<string, ArtifactItem>();
  for (const message of messages) {
    if (message.role !== "assistant") {
      continue;
    }
    let context = message_artifact_context(message);
    for (const block of message.content) {
      context = collect_content_block_artifacts(artifacts, block, context);
    }
  }

  return Array.from(artifacts.values())
    .sort((a, b) => b.created_at - a.created_at || b.id.localeCompare(a.id));
}
