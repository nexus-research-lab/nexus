import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import type { SkillInfo } from "@/types/capability/skill";

type Translate = I18nContextValue["t"];
type SkillCategory = Pick<SkillInfo, "category_key" | "category_name">;

const SKILL_CATEGORY_LABEL_KEYS = {
  "builtin-misc": "capability.skill_category.builtin_misc",
  "content-docs": "capability.skill_category.content_docs",
  "custom-imports": "capability.skill_category.custom_imports",
  "data-automation": "capability.skill_category.data_automation",
  "design-frontend": "capability.skill_category.design_frontend",
  "ecommerce-growth": "capability.skill_category.ecommerce_growth",
  presentation: "capability.skill_category.presentation",
  "programming-development": "capability.skill_category.programming_development",
  "research-analysis": "capability.skill_category.research_analysis",
  "system-builtins": "capability.skill_category.system_builtins",
} as const satisfies Readonly<Record<string, TranslationKey>>;

type LocalizedSkillCategoryKey = keyof typeof SKILL_CATEGORY_LABEL_KEYS;

function isLocalizedSkillCategoryKey(
  key: string,
): key is LocalizedSkillCategoryKey {
  return Object.hasOwn(SKILL_CATEGORY_LABEL_KEYS, key);
}

/** 只翻译 Nexus 已知分类，用户自定义分类保持真实名称。 */
export function getSkillCategoryLabel(
  skill: SkillCategory,
  t: Translate,
): string {
  const key = skill.category_key.trim().toLocaleLowerCase();
  return isLocalizedSkillCategoryKey(key)
    ? t(SKILL_CATEGORY_LABEL_KEYS[key])
    : skill.category_name;
}
