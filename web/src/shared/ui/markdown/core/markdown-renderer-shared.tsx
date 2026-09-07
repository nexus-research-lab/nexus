/**
 * INPUT: Markdown 内容、受控文件解析与流式状态。
 * OUTPUT: 通用插件、正文配方和保留受保护区域的规范化文本。
 * POS: 共享 Markdown 解析配置；公式源文排除预处理改写，领域链接协议放行由消费侧维护。
 */

"use client";

import { remarkLatexMath, rehypeMathViewport } from "./markdown-math";
import { isInMarkdownMathRange, readMarkdownMathRanges } from "./markdown-math-ranges";
import rehypeKatex from "rehype-katex";
import remarkBreaks from "remark-breaks";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import type { PluggableList } from "unified";

import { findOpenMarkdownFenceLanguage, readMarkdownFenceMarker } from "./markdown-fence";
import { stabilizeStreamingMarkdownUrlTail } from "./markdown-link-model";
import {
  remarkInlineHtmlTags,
  remarkMarkdownBreaks,
  remarkMixedScript,
} from "./markdown-text-plugins";
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
  remarkLatexMath,
  remarkGfm,
  remarkMarkdownBreaks,
  remarkInlineHtmlTags,
  remarkMixedScript,
  remarkBreaks,
];
export const REHYPE_PLUGINS: PluggableList = [
  [rehypeKatex, { trust: false, errorColor: "var(--text-muted)" }],
  rehypeMathViewport,
];

export const MARKDOWN_BODY_CLASS_NAME = "nexus-chat-markdown nexus-markdown-body message-cjk-font w-full min-w-0 max-w-full overflow-x-hidden text-md leading-[1.65rem] text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic";
export const MARKDOWN_SUMMARY_CLASS_NAME = "nexus-chat-markdown message-cjk-font w-full min-w-0 max-w-full overflow-hidden text-base leading-[1.5] text-(--text-strong) [&_strong]:font-semibold [&_strong]:text-(--text-strong) [&_em]:italic";

export function normalizeMarkdownContent(
  content: string,
  resolveFilePath: ResolveWorkspaceFilePath,
  onOpenWorkspaceFile?: (path: string) => void,
  options: NormalizeMarkdownContentOptions = {},
): string {
  const mathRanges = readMarkdownMathRanges(content, (offset) => isInsideMarkdownProtectedRegion(content, offset));
  const escapedContent = escapeIdentifierAsterisksBeforeBrackets(content, (offset) => isInMarkdownMathRange(mathRanges, offset));
  const escapedMathRanges = readMarkdownMathRanges(escapedContent, (offset) => isInsideMarkdownProtectedRegion(escapedContent, offset));
  const normalizedContent = stabilizeStreamingMarkdownUrlTail(
    escapedContent,
    Boolean(options.is_streaming),
    (offset) => isInsideMarkdownProtectedRegion(escapedContent, offset) || isInMarkdownMathRange(escapedMathRanges, offset),
  );
  const normalizedMathRanges = normalizedContent === escapedContent ? escapedMathRanges
    : readMarkdownMathRanges(normalizedContent, (offset) => isInsideMarkdownProtectedRegion(normalizedContent, offset));
  return normalizedContent.replace(WORKSPACE_FILE_PATTERN, (match, offset: number) => {
    if (
      isInMarkdownMathRange(normalizedMathRanges, offset) ||
      isInsideMarkdownProtectedRegion(normalizedContent, offset) ||
      isInsideMarkdownLinkDestination(normalizedContent, offset, match.length)
    ) {
      return match;
    }
    const resolvedPath = resolveWorkspaceArtifactPath(match, resolveFilePath);
    return resolvedPath && onOpenWorkspaceFile ? `\`${match}\`` : match;
  });
}

function escapeIdentifierAsterisksBeforeBrackets(content: string, isMath: (offset: number) => boolean): string {
  let openFence: { marker: "`" | "~"; length: number } | null = null;
  let cursor = 0;

  return (content.match(/[^\n]*(?:\n|$)/g)?.filter((line) => line.length > 0) ?? [])
    .map((line) => {
      const lineOffset = cursor;
      cursor += line.length;
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

      return escapeInlineMarkdownIdentifierAsterisks(line, (offset) => isMath(lineOffset + offset));
    })
    .join("");
}

function escapeInlineMarkdownIdentifierAsterisks(line: string, isMath: (offset: number) => boolean): string {
  let inCode = false;
  let codeMarker = "";
  let cursor = 0;

  return line
    .split(/(`+)/)
    .map((part) => {
      const partOffset = cursor;
      cursor += part.length;
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
        : part.replace(MARKDOWN_IDENTIFIER_ASTERISK_BEFORE_BRACKET_PATTERN, (match, offset: number) => isMath(partOffset + offset) ? match : "\\*");
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
