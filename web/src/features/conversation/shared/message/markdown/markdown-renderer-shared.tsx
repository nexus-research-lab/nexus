/**
 * =====================================================
 * @File   : markdown-renderer-shared.tsx
 * @Date   : 2026-04-05 15:26
 * @Author : leemysw
 * 2026-04-05 15:26   Create
 * =====================================================
 */

"use client";

import rehypeKatex from "rehype-katex";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import { find_open_markdown_fence_language, read_markdown_fence_marker } from "./markdown-fence";
import { remarkInlineHtmlTags, remarkMarkdownBreaks } from "./markdown-text-plugins";
import {
  resolve_workspace_artifact_path,
  type ResolveWorkspaceFilePath,
} from "./markdown-workspace-artifacts";

interface NormalizeMarkdownContentOptions {
  is_streaming?: boolean;
}

const WORKSPACE_FILE_PATTERN = /([A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,10})/g;
const MARKDOWN_IDENTIFIER_ASTERISK_BEFORE_BRACKET_PATTERN = /(?<=[\p{L}\p{N}_./-])\*(?=[(\[（［])/gu;
const STREAMING_URL_TAIL_PATTERN = /(?:https?:\/\/|www\.|mailto:)[^\s<>"'，。；！？]*$/iu;
const STREAMING_MARKDOWN_LINK_DESTINATION_TAIL_PATTERN = /(\[[^\]\n]{0,180}\]\()((?:https?:\/\/|www\.|mailto:)[^\s)]*)$/iu;
const STREAMING_AUTOLINK_TAIL_PATTERN = /<((?:https?:\/\/|www\.|mailto:)[^\s>]*)$/iu;

// 数学语法必须先于 GFM 表格解析，避免公式里的 `|` 被误判为列分隔符。
export const MARKDOWN_PLUGINS = [
  remarkMath,
  remarkGfm,
  remarkMarkdownBreaks,
  remarkInlineHtmlTags,
  remarkBreaks,
];
export const REHYPE_PLUGINS = [rehypeKatex];
export const MARKDOWN_BODY_CLASS_NAME = "nexus-chat-markdown message-cjk-font w-full min-w-0 max-w-full overflow-x-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic [&_hr]:my-4 [&_hr]:border-(--divider-subtle-color)";
export const MARKDOWN_SUMMARY_CLASS_NAME = "nexus-chat-markdown message-cjk-font w-full min-w-0 max-w-full overflow-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic";

export function normalize_markdown_content(
  content: string,
  resolve_file_path: ResolveWorkspaceFilePath,
  on_open_workspace_file?: (path: string) => void,
  options: NormalizeMarkdownContentOptions = {},
): string {
  const normalized_content = stabilize_streaming_url_tail(
    escape_identifier_asterisks_before_brackets(content),
    Boolean(options.is_streaming),
  );
  return normalized_content.replace(WORKSPACE_FILE_PATTERN, (match, offset: number) => {
    if (
      is_inside_markdown_protected_region(normalized_content, offset) ||
      is_inside_markdown_link_destination(normalized_content, offset, match.length)
    ) {
      return match;
    }
    const resolved_path = resolve_workspace_artifact_path(match, resolve_file_path);
    return resolved_path && on_open_workspace_file ? `\`${match}\`` : match;
  });
}

function stabilize_streaming_url_tail(content: string, is_streaming: boolean): string {
  if (!is_streaming || !content) {
    return content;
  }

  const markdown_link_match = STREAMING_MARKDOWN_LINK_DESTINATION_TAIL_PATTERN.exec(content);
  if (markdown_link_match?.[1] && markdown_link_match[2]) {
    const url_offset = content.length - markdown_link_match[2].length;
    if (!is_inside_markdown_protected_region(content, url_offset)) {
      return `${content.slice(0, url_offset)}${escape_markdown_url_tail(markdown_link_match[2])}`;
    }
  }

  const autolink_match = STREAMING_AUTOLINK_TAIL_PATTERN.exec(content);
  if (autolink_match?.[1]) {
    const url_offset = content.length - autolink_match[1].length;
    if (!is_inside_markdown_protected_region(content, url_offset)) {
      return `${content.slice(0, url_offset - 1)}&lt;${escape_markdown_url_tail(autolink_match[1])}`;
    }
  }

  const url_match = STREAMING_URL_TAIL_PATTERN.exec(content);
  if (!url_match?.[0]) {
    return content;
  }

  const url_offset = content.length - url_match[0].length;
  if (is_inside_markdown_protected_region(content, url_offset)) {
    return content;
  }

  // 中文注释：流式尾巴上的 URL 大概率还没写完，先打断 GFM 自动链接，等空白/换行收尾后再恢复为真实链接。
  return `${content.slice(0, url_offset)}${escape_markdown_url_tail(url_match[0])}`;
}

function escape_markdown_url_tail(value: string): string {
  return value.replace(/([.:<>()[\]])/g, "\\$1");
}

function escape_identifier_asterisks_before_brackets(content: string): string {
  let open_fence: { marker: "`" | "~"; length: number } | null = null;

  return (content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [])
    .map((line) => {
      const fence_marker = read_markdown_fence_marker(line);

      if (open_fence) {
        if (
          fence_marker &&
          fence_marker.marker === open_fence.marker &&
          fence_marker.length >= open_fence.length
        ) {
          open_fence = null;
        }
        return line;
      }

      if (fence_marker) {
        open_fence = fence_marker;
        return line;
      }

      return escape_inline_markdown_identifier_asterisks(line);
    })
    .join("");
}

function escape_inline_markdown_identifier_asterisks(line: string): string {
  let in_code = false;
  let code_marker = "";

  return line
    .split(/(`+)/)
    .map((part) => {
      if (/^`+$/.test(part)) {
        if (!in_code) {
          in_code = true;
          code_marker = part;
        } else if (part.length === code_marker.length) {
          in_code = false;
          code_marker = "";
        }
        return part;
      }

      return in_code
        ? part
        : part.replace(MARKDOWN_IDENTIFIER_ASTERISK_BEFORE_BRACKET_PATTERN, "\\*");
    })
    .join("");
}

function is_inside_inline_code(content: string, offset: number): boolean {
  const before = content.slice(0, offset);
  return (before.match(/`/g)?.length ?? 0) % 2 === 1;
}

function is_inside_markdown_protected_region(content: string, offset: number): boolean {
  return (
    is_inside_inline_code(content, offset) ||
    find_open_markdown_fence_language(content.slice(0, offset)) !== null
  );
}

function is_inside_markdown_link_destination(
  content: string,
  offset: number,
  length: number,
): boolean {
  const before = content.slice(0, offset);
  const open_paren_index = before.lastIndexOf("(");
  if (open_paren_index < 0 || before.lastIndexOf(")") > open_paren_index) {
    return false;
  }

  const before_destination = before.slice(0, open_paren_index).trimEnd();
  if (!before_destination.endsWith("]")) {
    return false;
  }

  const after = content.slice(offset + length);
  const close_paren_index = after.indexOf(")");
  const newline_index = after.search(/\r?\n/);
  return close_paren_index >= 0 && (newline_index < 0 || close_paren_index < newline_index);
}
