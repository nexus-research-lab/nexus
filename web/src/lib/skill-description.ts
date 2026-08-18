import type { TranslationKey } from "@/shared/i18n/messages";
import type { SkillInfo } from "@/types/capability/skill";

type Translate = (key: TranslationKey) => string;

const NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME = {
  "goal-manager": "capability.skill_description.goal_manager",
  "ima-skill": "capability.skill_description.ima",
  imagegen: "capability.skill_description.imagegen",
  "room-playbook": "capability.skill_description.room_playbook",
  "wechat-article-search": "capability.skill_description.wechat_article_search",
  "werewolf-6p": "capability.skill_description.werewolf_6p",
} as const satisfies Readonly<Record<string, TranslationKey>>;

type LocalizedSkillName = keyof typeof NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME;

function isLocalizedSkillName(name: string): name is LocalizedSkillName {
  return Object.hasOwn(NEXUS_SKILL_DESCRIPTION_KEY_BY_NAME, name);
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
