import { getSkillCategoryLabel } from "@/lib/skill-category";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type {
  SkillAgentBinding,
  SkillDetail,
  SkillInfo,
  SkillSourceType,
} from "@/types/capability/skill";

export type SkillDetailSnapshot =
  | { errorMessage: null; skill: null; status: "loading" }
  | { errorMessage: string; skill: null; status: "error" }
  | { errorMessage: null; skill: SkillDetail; status: "ready" };

interface SkillSourcePresentation {
  labelKey: TranslationKey;
}

interface SkillDetailBadge {
  key: string;
  label: string;
  tone: "default" | "warning";
}

export interface SkillDetailPresentation {
  avatarSeed: string;
  badges: SkillDetailBadge[];
  canDelete: boolean;
  canUpdate: boolean;
  description: string;
  displayName: string;
  locked: boolean;
  readmeMarkdown: string;
  scope: SkillInfo["scope"];
  sourceUrl: string | null;
}

export interface SkillAgentBindingPresentation {
  description: string;
  status: string;
  switchLabel: string;
}

type SkillDetailLocalization = Pick<I18nContextValue, "t">;

const SKILL_SOURCE_PRESENTATION: Record<
  SkillSourceType,
  SkillSourcePresentation
> = {
  builtin: {
    labelKey: "capability.skills_source.builtin",
  },
  external: {
    labelKey: "capability.skills_source.user_import",
  },
  system: {
    labelKey: "capability.skills_source.system",
  },
  workspace: {
    labelKey: "capability.skills_source.agent_local",
  },
};

function getSkillSourcePresentation(skill: SkillInfo): SkillSourcePresentation {
  if (skill.source_type === "builtin") {
    if (skill.source_kind === "nexus_platform") {
      return { labelKey: "capability.skills_source.nexus_library" };
    }
    if (skill.source_kind === "user_global") {
      return { labelKey: "capability.skills_source.user_global" };
    }
  }
  if (skill.origin_kind === "marketplace") {
    return { labelKey: "capability.skills_source.marketplace" };
  }
  if (skill.origin_kind === "user_import") {
    return { labelKey: "capability.skills_source.user_import" };
  }
  return SKILL_SOURCE_PRESENTATION[skill.source_type];
}

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
  description: string,
  localization: SkillDetailLocalization,
): SkillDetailPresentation {
  const source = getSkillSourcePresentation(skill);
  const sourceLabel = localization.t(source.labelKey);
  const categoryLabel = getSkillCategoryLabel(skill, localization.t);
  const displayName = skill.title || skill.name;
  const optionalFlagBadges: Array<SkillDetailBadge | false> = [
    skill.scope === "room" && {
      key: "room",
      label: localization.t("capability.skills_badge.room_only"),
      tone: "default" as const,
    },
    skill.has_update && {
      key: "update",
      label: localization.t("capability.skills_update_available"),
      tone: "warning" as const,
    },
    skill.locked && {
      key: "locked",
      label: localization.t("capability.skills_badge.system_locked"),
      tone: "warning" as const,
    },
  ];
  const flagBadges = optionalFlagBadges.filter(isSkillDetailBadge);
  const sourceBadges: SkillDetailBadge[] =
    sourceLabel.trim() === categoryLabel.trim()
      ? []
      : [{ key: "source", label: sourceLabel, tone: "default" }];
  const badges: SkillDetailBadge[] = [
    { key: "category", label: categoryLabel, tone: "default" },
    ...sourceBadges,
    {
      key: "version",
      label: localization.t("capability.skills_badge.version", {
        version: skill.version || "unknown",
      }),
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
    avatarSeed: skill.name,
    badges,
    canDelete: skill.deletable,
    canUpdate: skill.source_type === "external" && skill.has_update,
    description,
    displayName,
    locked: skill.locked,
    readmeMarkdown: skill.readme_markdown,
    scope: skill.scope,
    sourceUrl: getHttpSourceUrl(skill.source_ref),
  };
}

export function buildSkillAgentBindingPresentation(
  binding: SkillAgentBinding,
  locked: boolean,
  t: I18nContextValue["t"],
): SkillAgentBindingPresentation {
  const switchLabel = t("capability.skills_detail_binding_switch", {
    name: binding.agent_name,
  });
  if (locked) {
    return {
      description: t("capability.skills_detail_binding_system_managed"),
      status: binding.enabled
        ? t("capability.skills_detail_binding_enabled")
        : t("capability.skills_detail_binding_disabled"),
      switchLabel,
    };
  }
  if (!binding.available) {
    return {
      description: t("capability.skills_detail_binding_unavailable"),
      status: t("capability.skills_detail_binding_cannot_enable"),
      switchLabel,
    };
  }
  return {
    description: binding.is_main
      ? t("capability.skills_detail_binding_main_agent")
      : t("capability.skills_detail_binding_independent"),
    status: binding.enabled
      ? t("capability.skills_detail_binding_enabled")
      : t("capability.skills_detail_binding_enable"),
    switchLabel,
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
