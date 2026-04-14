"use client";

import { MarkdownRendererContent } from "@/features/conversation-shared/message/markdown-renderer-content";
import { cn } from "@/lib/utils";

const SKILL_MARKDOWN_CLASS_NAME =
  "[&_h1:first-child]:mt-0 [&_h2:first-child]:mt-0 [&_h3:first-child]:mt-0 [&_p:first-child]:mt-0";

interface SkillMarkdownProps {
  markdown: string;
  title?: string;
  description?: string;
  class_name?: string;
}

function normalize_plain_text(value: string): string {
  return value
    .replace(/\r\n/g, "\n")
    .replace(/[`*_>#~\-]/g, " ")
    .replace(/\[(.*?)\]\((.*?)\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim()
    .toLocaleLowerCase();
}

function strip_leading_duplicate_content(markdown: string, title?: string, description?: string): string {
  const normalized_markdown = markdown.replace(/^\uFEFF/, "").trim();
  if (!normalized_markdown) {
    return "";
  }

  let next_markdown = normalized_markdown;
  const normalized_title = title ? normalize_plain_text(title) : "";
  const normalized_description = description ? normalize_plain_text(description) : "";

  const frontmatter_match = next_markdown.match(/^---\s*\n[\s\S]*?\n---\s*(?:\n+|$)/);
  if (frontmatter_match) {
    next_markdown = next_markdown.slice(frontmatter_match[0].length).trimStart();
  }

  const heading_match = next_markdown.match(/^#\s+(.+?)\n+/);
  if (heading_match && normalized_title && normalize_plain_text(heading_match[1]) === normalized_title) {
    next_markdown = next_markdown.slice(heading_match[0].length).trimStart();
  }

  // 中文注释：很多 Skill README 的首段会把 description 原样再写一遍，
  // 这里在详情弹窗里裁掉这段重复导语，保留正文结构不变。
  if (normalized_description) {
    const first_block_match = next_markdown.match(/^([\s\S]*?)(?:\n\s*\n|$)/);
    const first_block = first_block_match?.[1]?.trim() ?? "";
    if (
      first_block
      && !/^(#|>|-|[*]|\d+\.)/.test(first_block)
      && normalize_plain_text(first_block) === normalized_description
    ) {
      next_markdown = next_markdown.slice(first_block_match![0].length).trimStart();
    }
  }

  return next_markdown;
}

export function SkillMarkdown({ markdown, title, description, class_name }: SkillMarkdownProps) {
  const normalized_markdown = strip_leading_duplicate_content(markdown, title, description);

  return (
    <MarkdownRendererContent
      class_name={cn(SKILL_MARKDOWN_CLASS_NAME, class_name)}
      content={normalized_markdown || markdown}
    />
  );
}
