// INPUT: Skill 包说明、标题和简介。
// OUTPUT: 归一化后的普通 Markdown，保留包内相对图片和安全外链。
// POS: Skill 说明消费侧；没有 Workspace 来源，不将当前 Agent 作为资源基址。
"use client";

import { UiMarkdownContent } from "@/shared/ui/markdown/markdown-content";
import { cn } from "@/shared/ui/class-name";

import { normalizeSkillMarkdownContent } from "./skill-detail-model";

const SKILL_MARKDOWN_CLASS_NAME =
  "[&_h1:first-child]:mt-0 [&_h2:first-child]:mt-0 [&_h3:first-child]:mt-0 [&_p:first-child]:mt-0";

interface SkillMarkdownProps {
  markdown: string;
  title?: string;
  description?: string;
  className?: string;
}

export function SkillMarkdown({ markdown, title, description, className }: SkillMarkdownProps) {
  const normalizedMarkdown = normalizeSkillMarkdownContent(
    markdown,
    title,
    description,
  );

  return (
    <UiMarkdownContent
      className={cn(SKILL_MARKDOWN_CLASS_NAME, className)}
      content={normalizedMarkdown || markdown}
    />
  );
}
