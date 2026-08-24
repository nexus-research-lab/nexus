/**
 * INPUT: Skill 目录对象与当前语言翻译函数。
 * OUTPUT: Nexus 随产品 Skill 的用户可读标题和短摘要。
 * POS: Skill 原始元数据到目录/详情文案的唯一只读投影。
 */
import type { TranslationKey } from "@/shared/i18n/messages";
import type { SkillInfo } from "@/types/capability/skill";

type Translate = (key: TranslationKey) => string;

const NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME = {
  automation: "capability.skill_description.automation",
  docx: "capability.skill_description.docx",
  "execution-orchestrator": "capability.skill_description.execution_orchestrator",
  "goal-manager": "capability.skill_description.goal_manager",
  "ima-skill": "capability.skill_description.ima",
  imagegen: "capability.skill_description.imagegen",
  "nexus-configuration": "capability.skill_description.nexus_configuration",
  "nexus-product-guide": "capability.skill_description.nexus_product_guide",
  pdf: "capability.skill_description.pdf",
  pptx: "capability.skill_description.pptx",
  "room-playbook": "capability.skill_description.room_playbook",
  "wechat-article-search": "capability.skill_description.wechat_article_search",
  "werewolf-6p": "capability.skill_description.werewolf_6p",
  xlsx: "capability.skill_description.xlsx",
} as const satisfies Readonly<Record<string, TranslationKey>>;

const NEXUS_SKILL_TITLE_KEY_BY_NAME = {
  automation: "capability.skill_title.automation",
  docx: "capability.skill_title.docx",
  "execution-orchestrator": "capability.skill_title.execution_orchestrator",
  "goal-manager": "capability.skill_title.goal_manager",
  imagegen: "capability.skill_title.imagegen",
  "nexus-configuration": "capability.skill_title.nexus_configuration",
  "nexus-product-guide": "capability.skill_title.nexus_product_guide",
  pdf: "capability.skill_title.pdf",
  pptx: "capability.skill_title.pptx",
  "room-playbook": "capability.skill_title.room_playbook",
  visualize: "capability.skill_title.visualize",
  "werewolf-6p": "capability.skill_title.werewolf_6p",
  xlsx: "capability.skill_title.xlsx",
} as const satisfies Readonly<Record<string, TranslationKey>>;

type LocalizedSkillName = keyof typeof NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME;
type LocalizedSkillTitleName = keyof typeof NEXUS_SKILL_TITLE_KEY_BY_NAME;

function isLocalizedSkillName(name: string): name is LocalizedSkillName {
  return Object.hasOwn(NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME, name);
}

function isLocalizedSkillTitleName(name: string): name is LocalizedSkillTitleName {
  return Object.hasOwn(NEXUS_SKILL_TITLE_KEY_BY_NAME, name);
}

function isNexusBundledSkill(skill: SkillInfo): boolean {
  return skill.source_type === "system"
    || (
      skill.source_type === "builtin"
      && skill.source_kind === "nexus_platform"
    );
}

/** 只覆盖 Nexus 随产品提供的展示说明，不修改服务端 Skill 元数据。 */
export function getSkillDisplayDescription(
  skill: SkillInfo,
  t: Translate,
): string {
  const normalizedName = skill.name.trim().toLocaleLowerCase();
  if (!isNexusBundledSkill(skill) || !isLocalizedSkillName(normalizedName)) {
    return skill.description;
  }
  return t(NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME[normalizedName]);
}

/** 只覆盖 Nexus 随产品提供的展示标题，不修改服务端 Skill 元数据。 */
export function getSkillDisplayTitle(
  skill: SkillInfo,
  t: Translate,
): string {
  const normalizedName = skill.name.trim().toLocaleLowerCase();
  if (!isNexusBundledSkill(skill) || !isLocalizedSkillTitleName(normalizedName)) {
    return skill.title || skill.name;
  }
  return t(NEXUS_SKILL_TITLE_KEY_BY_NAME[normalizedName]);
}
