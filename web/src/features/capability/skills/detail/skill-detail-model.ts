import type {
  SkillDetail,
  SkillSourceType,
} from "@/types/capability/skill";

export type SkillDetailSnapshot =
  | { errorMessage: null; skill: null; status: "loading" }
  | { errorMessage: string; skill: null; status: "error" }
  | { errorMessage: null; skill: SkillDetail; status: "ready" };

interface SkillSourcePresentation {
  iconClassName: string;
  label: string;
}

interface SkillLockPresentation {
  icon: SkillDetailPresentation["icon"];
  iconClassName: string | null;
}

interface SkillDetailBadge {
  key: string;
  label: string;
  tone: "default" | "warning";
}

export interface SkillDetailPresentation {
  badges: SkillDetailBadge[];
  canDelete: boolean;
  canUpdate: boolean;
  description: string;
  displayName: string;
  icon: "lock" | "puzzle";
  iconClassName: string;
  readmeMarkdown: string;
  sourceUrl: string | null;
}

const SKILL_SOURCE_PRESENTATION: Record<
  SkillSourceType,
  SkillSourcePresentation
> = {
  builtin: {
    iconClassName: "text-(--icon-default)",
    label: "内置推荐",
  },
  external: {
    iconClassName: "text-(--status-info-soft-text)",
    label: "用户导入",
  },
  system: {
    iconClassName: "text-(--icon-default)",
    label: "系统内置",
  },
  workspace: {
    iconClassName: "text-(--icon-default)",
    label: "工作区技能",
  },
};

const SKILL_LOCK_PRESENTATION: Record<"false" | "true", SkillLockPresentation> = {
  false: { icon: "puzzle", iconClassName: null },
  true: { icon: "lock", iconClassName: "text-(--warning)" },
};

const HTTP_URL_PATTERN = /^https?:\/\//;
const SKILL_FRONTMATTER_PATTERN = /^---\s*\n[\s\S]*?\n---\s*(?:\n+|$)/;
const SKILL_HEADING_PATTERN = /^#\s+(.+?)\n+/;
const SKILL_FIRST_BLOCK_PATTERN = /^([\s\S]*?)(?:\n\s*\n|$)/;
const SKILL_STRUCTURED_BLOCK_PATTERN = /^(#|>|-|[*]|\d+\.)/;

interface SkillMarkdownContext {
  description: string;
  title: string;
}

type SkillMarkdownTransform = (
  markdown: string,
  context: SkillMarkdownContext,
) => string;

const SKILL_MARKDOWN_TRANSFORMS: readonly SkillMarkdownTransform[] = [
  stripSkillFrontmatter,
  stripDuplicateSkillTitle,
  stripDuplicateSkillDescription,
];

export function buildSkillDetailPresentation(
  skill: SkillDetail,
): SkillDetailPresentation {
  const source = SKILL_SOURCE_PRESENTATION[skill.source_type];
  const lock = SKILL_LOCK_PRESENTATION[String(skill.locked) as "false" | "true"];
  const displayName = skill.title || skill.name;
  const optionalFlagBadges: Array<SkillDetailBadge | false> = [
    skill.has_update && {
      key: "update",
      label: "有更新",
      tone: "warning" as const,
    },
    skill.locked && {
      key: "locked",
      label: "系统锁定",
      tone: "warning" as const,
    },
  ];
  const flagBadges = optionalFlagBadges.filter(isSkillDetailBadge);
  const badges: SkillDetailBadge[] = [
    { key: "category", label: skill.category_name, tone: "default" },
    { key: "source", label: source.label, tone: "default" },
    {
      key: "version",
      label: `版本 ${skill.version || "unknown"}`,
      tone: "default",
    },
    ...flagBadges,
    ...skill.tags.map((tag) => ({
      key: `tag:${tag}`,
      label: tag,
      tone: "default" as const,
    })),
  ];

  return {
    badges,
    canDelete: skill.deletable,
    canUpdate: skill.source_type === "external" && skill.has_update,
    description: skill.description || "暂无描述",
    displayName,
    icon: lock.icon,
    iconClassName: lock.iconClassName || source.iconClassName,
    readmeMarkdown: skill.readme_markdown,
    sourceUrl: getHttpSourceUrl(skill.source_ref),
  };
}

function isSkillDetailBadge(
  value: SkillDetailBadge | false,
): value is SkillDetailBadge {
  return Boolean(value);
}

function getHttpSourceUrl(value: string): string | null {
  return value && HTTP_URL_PATTERN.test(value) ? value : null;
}

export function getSkillDetailSnapshotTitle(
  snapshot: SkillDetailSnapshot,
): string | null {
  return snapshot.status === "ready"
    ? snapshot.skill.title || snapshot.skill.name
    : null;
}

export function normalizeSkillMarkdownContent(
  markdown: string,
  title?: string,
  description?: string,
): string {
  const normalizedMarkdown = markdown.replace(/^\uFEFF/, "").trim();
  if (!normalizedMarkdown) {
    return "";
  }
  const context = {
    description: description ? normalizeSkillPlainText(description) : "",
    title: title ? normalizeSkillPlainText(title) : "",
  };
  return SKILL_MARKDOWN_TRANSFORMS.reduce(
    (content, transform) => transform(content, context),
    normalizedMarkdown,
  );
}

function normalizeSkillPlainText(value: string): string {
  return value
    .replace(/\r\n/g, "\n")
    .replace(/[`*_>#~\-]/g, " ")
    .replace(/\[(.*?)\]\((.*?)\)/g, "$1")
    .replace(/\s+/g, " ")
    .trim()
    .toLocaleLowerCase();
}

function stripSkillFrontmatter(markdown: string): string {
  const match = markdown.match(SKILL_FRONTMATTER_PATTERN);
  return match ? markdown.slice(match[0].length).trimStart() : markdown;
}

function stripDuplicateSkillTitle(
  markdown: string,
  context: SkillMarkdownContext,
): string {
  if (!context.title) {
    return markdown;
  }
  const match = markdown.match(SKILL_HEADING_PATTERN);
  if (!match || normalizeSkillPlainText(match[1]) !== context.title) {
    return markdown;
  }
  return markdown.slice(match[0].length).trimStart();
}

function stripDuplicateSkillDescription(
  markdown: string,
  context: SkillMarkdownContext,
): string {
  if (!context.description) {
    return markdown;
  }
  const match = markdown.match(SKILL_FIRST_BLOCK_PATTERN);
  const firstBlock = match?.[1]?.trim() ?? "";
  if (!match || !isDuplicateDescriptionBlock(firstBlock, context.description)) {
    return markdown;
  }
  return markdown.slice(match[0].length).trimStart();
}

function isDuplicateDescriptionBlock(
  block: string,
  normalizedDescription: string,
): boolean {
  return Boolean(block)
    && !SKILL_STRUCTURED_BLOCK_PATTERN.test(block)
    && normalizeSkillPlainText(block) === normalizedDescription;
}
