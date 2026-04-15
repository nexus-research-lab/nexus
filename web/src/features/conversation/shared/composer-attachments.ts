/**
# !/usr/bin/env xx
# -*- coding: utf-8 -*-
# =====================================================
# @File   ：composer-attachments.ts
# @Date   ：2026-04-12 17:46
# @Author ：leemysw
# 2026-04-12 17:46   Create
# =====================================================
*/

import { create_workspace_entry_api } from "@/lib/agent-manage-api";

export interface PreparedComposerAttachment {
  file_name: string;
  workspace_path: string;
  excerpt: string;
  truncated: boolean;
}

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
]);

const SUPPORTED_TEXT_MIME_PREFIXES = [
  "text/",
];

const SUPPORTED_TEXT_MIME_TYPES = new Set([
  "application/json",
  "application/ld+json",
  "application/xml",
  "application/x-yaml",
  "application/yaml",
]);

const MAX_ATTACHMENT_SIZE_BYTES = 256 * 1024;
const MAX_ATTACHMENT_PREVIEW_CHARS = 12_000;
const ATTACHMENT_DIRECTORY = ".nexus/attachments";

export const COMPOSER_ATTACHMENT_ACCEPT =
  ".txt,.md,.markdown,.json,.jsonl,.yaml,.yml,.csv,.ts,.tsx,.js,.jsx,.mjs,.cjs,.py,.java,.go,.rs,.rb,.php,.sh,.bash,.zsh,.sql,.xml,.html,.css,.scss,.less,.log,.ini,.toml,.env,.conf";

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

export function get_attachment_rejection_reason(file: File): string | null {
  if (!is_supported_text_attachment(file)) {
    return `暂仅支持文本附件：${file.name}`;
  }

  if (file.size > MAX_ATTACHMENT_SIZE_BYTES) {
    return `附件过大，请控制在 256KB 内：${file.name}`;
  }

  return null;
}

function build_attachment_path(file_name: string, index: number): string {
  const safe_name = sanitize_attachment_name(file_name);
  const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
  return `${ATTACHMENT_DIRECTORY}/${timestamp}-${index + 1}-${safe_name}`;
}

export async function prepare_workspace_text_attachments(
  agent_id: string,
  files: File[],
): Promise<PreparedComposerAttachment[]> {
  const next_attachments: PreparedComposerAttachment[] = [];

  for (const [index, file] of files.entries()) {
    const rejection_reason = get_attachment_rejection_reason(file);
    if (rejection_reason) {
      throw new Error(rejection_reason);
    }

    // 中文注释：附件当前仅走“文本文件同步到工作区”这条真实链路，
    // 避免前端伪造图片/二进制上传能力而后端实际上无法消费。
    const content = await file.text();
    const workspace_path = build_attachment_path(file.name, index);
    const created_entry = await create_workspace_entry_api(
      agent_id,
      workspace_path,
      "file",
      content,
    );
    const excerpt = content.slice(0, MAX_ATTACHMENT_PREVIEW_CHARS);

    next_attachments.push({
      file_name: file.name,
      workspace_path: created_entry.path,
      excerpt,
      truncated: content.length > MAX_ATTACHMENT_PREVIEW_CHARS,
    });
  }

  return next_attachments;
}
