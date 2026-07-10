"use client";

import { MarkdownRendererContent } from "@/features/conversation/shared/message/markdown/markdown-renderer-content";
import { cn } from "@/lib/utils";

const SKILL_MARKDOWN_CLASS_NAME =
  "[&_h1:first-child]:mt-0 [&_h2:first-child]:mt-0 [&_h3:first-child]:mt-0 [&_p:first-child]:mt-0";

interface SkillMarkdownProps {
  markdown: string;
  title?: string;
  description?: string;
  className?: string;
}

function normalizePlainText(value: string): string {
  return value
    .replace(/\r\n/g, "\n")
    .replace(/[`*_>#~\-]/g, " ")
    .replace(/\[(.*?)\]\((.*?)\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim()
    .toLocaleLowerCase();
}

function stripLeadingDuplicateContent(markdown: string, title?: string, description?: string): string {
  const normalizedMarkdown = markdown.replace(/^\uFEFF/, "").trim();
  if (!normalizedMarkdown) {
    return "";
  }

  let nextMarkdown = normalizedMarkdown;
  const normalizedTitle = title ? normalizePlainText(title) : "";
  const normalizedDescription = description ? normalizePlainText(description) : "";

  const frontmatterMatch = nextMarkdown.match(/^---\s*\n[\s\S]*?\n---\s*(?:\n+|$)/);
  if (frontmatterMatch) {
    nextMarkdown = nextMarkdown.slice(frontmatterMatch[0].length).trimStart();
  }

  const headingMatch = nextMarkdown.match(/^#\s+(.+?)\n+/);
  if (headingMatch && normalizedTitle && normalizePlainText(headingMatch[1]) === normalizedTitle) {
    nextMarkdown = nextMarkdown.slice(headingMatch[0].length).trimStart();
  }

  // 很多 Skill README 的首段会把 description 原样再写一遍，
  // 这里在详情弹窗里裁掉这段重复导语，保留正文结构不变。
  if (normalizedDescription) {
    const firstBlockMatch = nextMarkdown.match(/^([\s\S]*?)(?:\n\s*\n|$)/);
    const firstBlock = firstBlockMatch?.[1]?.trim() ?? "";
    if (
      firstBlock
      && !/^(#|>|-|[*]|\d+\.)/.test(firstBlock)
      && normalizePlainText(firstBlock) === normalizedDescription
    ) {
      nextMarkdown = nextMarkdown.slice(firstBlockMatch![0].length).trimStart();
    }
  }

  return nextMarkdown;
}

export function SkillMarkdown({ markdown, title, description, className }: SkillMarkdownProps) {
  const normalizedMarkdown = stripLeadingDuplicateContent(markdown, title, description);

  return (
    <MarkdownRendererContent
      className={cn(SKILL_MARKDOWN_CLASS_NAME, className)}
      content={normalizedMarkdown || markdown}
    />
  );
}
