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
import { defaultUrlTransform } from "react-markdown";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";

import { findOpenMarkdownFenceLanguage, readMarkdownFenceMarker } from "./markdown-fence";
import { stabilizeStreamingMarkdownUrlTail } from "./markdown-link-model";
import { remarkInlineHtmlTags, remarkMarkdownBreaks } from "./markdown-text-plugins";
import {
  resolveWorkspaceArtifactPath,
  type ResolveWorkspaceFilePath,
} from "../workspace/markdown-workspace-artifact-model";

interface NormalizeMarkdownContentOptions {
  is_streaming?: boolean;
}

const WORKSPACE_FILE_PATTERN = /[A-Za-z0-9_./-]+\.[A-Za-z0-9]{1,10}/g;
const MARKDOWN_IDENTIFIER_ASTERISK_BEFORE_BRACKET_PATTERN = /(?<=[\p{L}\p{N}_./-])\*(?=[(\[（［])/gu;

// 数学语法必须先于 GFM 表格解析，避免公式里的 `|` 被误判为列分隔符。
export const MARKDOWN_PLUGINS = [
  remarkMath,
  remarkGfm,
  remarkMarkdownBreaks,
  remarkInlineHtmlTags,
  remarkBreaks,
];
export const REHYPE_PLUGINS = [rehypeKatex];

// mention 使用内部协议，必须显式加入白名单，否则 react-markdown 会把 href 清空成普通文本链接。
export function transformMarkdownUrl(value: string): string {
  return value.startsWith("agent-mention://") ? value : defaultUrlTransform(value);
}

export const MARKDOWN_BODY_CLASS_NAME = "nexus-chat-markdown message-cjk-font w-full min-w-0 max-w-full overflow-x-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic [&_hr]:my-4 [&_hr]:border-(--divider-subtle-color)";
export const MARKDOWN_SUMMARY_CLASS_NAME = "nexus-chat-markdown message-cjk-font w-full min-w-0 max-w-full overflow-hidden text-[15px] leading-7 text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic";

export function normalizeMarkdownContent(
  content: string,
  resolveFilePath: ResolveWorkspaceFilePath,
  onOpenWorkspaceFile?: (path: string) => void,
  options: NormalizeMarkdownContentOptions = {},
): string {
  const escapedContent = escapeIdentifierAsterisksBeforeBrackets(content);
  const normalizedContent = stabilizeStreamingMarkdownUrlTail(
    escapedContent,
    Boolean(options.is_streaming),
    (offset) => isInsideMarkdownProtectedRegion(escapedContent, offset),
  );
  return normalizedContent.replace(WORKSPACE_FILE_PATTERN, (match, offset: number) => {
    if (
      isInsideMarkdownProtectedRegion(normalizedContent, offset) ||
      isInsideMarkdownLinkDestination(normalizedContent, offset, match.length)
    ) {
      return match;
    }
    const resolvedPath = resolveWorkspaceArtifactPath(match, resolveFilePath);
    return resolvedPath && onOpenWorkspaceFile ? `\`${match}\`` : match;
  });
}

function escapeIdentifierAsterisksBeforeBrackets(content: string): string {
  let openFence: { marker: "`" | "~"; length: number } | null = null;

  return (content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [])
    .map((line) => {
      const fenceMarker = readMarkdownFenceMarker(line);

      if (openFence) {
        if (
          fenceMarker &&
          fenceMarker.marker === openFence.marker &&
          fenceMarker.length >= openFence.length
        ) {
          openFence = null;
        }
        return line;
      }

      if (fenceMarker) {
        openFence = fenceMarker;
        return line;
      }

      return escapeInlineMarkdownIdentifierAsterisks(line);
    })
    .join("");
}

function escapeInlineMarkdownIdentifierAsterisks(line: string): string {
  let inCode = false;
  let codeMarker = "";

  return line
    .split(/(`+)/)
    .map((part) => {
      if (/^`+$/.test(part)) {
        if (!inCode) {
          inCode = true;
          codeMarker = part;
        } else if (part.length === codeMarker.length) {
          inCode = false;
          codeMarker = "";
        }
        return part;
      }

      return inCode
        ? part
        : part.replace(MARKDOWN_IDENTIFIER_ASTERISK_BEFORE_BRACKET_PATTERN, "\\*");
    })
    .join("");
}

function isInsideInlineCode(content: string, offset: number): boolean {
  const before = content.slice(0, offset);
  return (before.match(/`/g)?.length ?? 0) % 2 === 1;
}

function isInsideMarkdownProtectedRegion(content: string, offset: number): boolean {
  return (
    isInsideInlineCode(content, offset) ||
    findOpenMarkdownFenceLanguage(content.slice(0, offset)) !== null
  );
}

function isInsideMarkdownLinkDestination(
  content: string,
  offset: number,
  length: number,
): boolean {
  const before = content.slice(0, offset);
  const openParenIndex = before.lastIndexOf("(");
  if (openParenIndex < 0 || before.lastIndexOf(")") > openParenIndex) {
    return false;
  }

  const beforeDestination = before.slice(0, openParenIndex).trimEnd();
  if (!beforeDestination.endsWith("]")) {
    return false;
  }

  const after = content.slice(offset + length);
  const closeParenIndex = after.indexOf(")");
  const newlineIndex = after.search(/\r?\n/);
  return closeParenIndex >= 0 && (newlineIndex < 0 || closeParenIndex < newlineIndex);
}
