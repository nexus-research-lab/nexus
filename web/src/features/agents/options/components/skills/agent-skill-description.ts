import type { TranslationKey } from "@/shared/i18n/messages";
import type { AgentSkillEntry } from "@/types/capability/skill";

type Translate = (key: TranslationKey) => string;

const SYSTEM_SKILL_DESCRIPTION_KEY_BY_NAME = {
  "goal-manager": "agent_options.skills.system_description.goal_manager",
  imagegen: "agent_options.skills.system_description.imagegen",
} as const satisfies Readonly<Record<string, TranslationKey>>;

type LocalizedSystemSkillName = keyof typeof SYSTEM_SKILL_DESCRIPTION_KEY_BY_NAME;

function isLocalizedSystemSkillName(
  name: string,
): name is LocalizedSystemSkillName {
  return Object.hasOwn(SYSTEM_SKILL_DESCRIPTION_KEY_BY_NAME, name);
}

/** 只覆盖系统内置 Skill 的展示说明，不修改服务端返回的真实元数据。 */
export function getAgentSkillDisplayDescription(
  skill: AgentSkillEntry,
  t: Translate,
): string {
  const normalizedName = skill.name.trim().toLocaleLowerCase();
  if (
    skill.source_type !== "system"
    || !isLocalizedSystemSkillName(normalizedName)
  ) {
    return skill.description;
  }
  return t(SYSTEM_SKILL_DESCRIPTION_KEY_BY_NAME[normalizedName]);
}
