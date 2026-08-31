// INPUT: Agent Skill 列表、搜索条件，以及 exact Agent+Skill 读取/修改失败事实。
// OUTPUT: 列表分组投影与不猜测 mutation 结果的可见恢复模型。
// POS: Agent Options Skill 子域的纯模型；不发请求、不自动重放开关。
import {
  getErrorMessage,
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import type { AgentSkillEntry } from "@/types/capability/skill";

export interface AgentSkillMutationTarget {
  agentId: string;
  desiredEnabled: boolean;
  skillName: string;
}

export interface AgentSkillsReadFailure {
  impact: string;
  message: string;
  nextStep: string;
  title: string;
}

export interface AgentSkillMutationFailure {
  blocksRepeat: boolean;
  effect: MutationFailureEffect;
  impact: string;
  message: string;
  nextStep: string;
  target: AgentSkillMutationTarget;
  title: string;
}

export function buildAgentSkillsReadFailure(
  error: unknown,
  t: I18nContextValue["t"],
): AgentSkillsReadFailure {
  return {
    impact: t("agent_options.skills.load_failed_impact"),
    message: getErrorMessage(error, t("agent_options.skills.load_failed")),
    nextStep: t("agent_options.skills.load_failed_next_step"),
    title: t("agent_options.skills.load_failed"),
  };
}

export function buildAgentSkillMutationFailure(
  error: unknown,
  target: AgentSkillMutationTarget,
  t: I18nContextValue["t"],
): AgentSkillMutationFailure {
  const failure = projectMutationFailure(
    error,
    t("agent_options.skills.toggle_failed"),
  );
  const notApplied = failure.effect === "not_applied";
  const committed = failure.effect === "committed";
  return {
    blocksRepeat: !notApplied,
    effect: failure.effect,
    impact: notApplied
      ? t("agent_options.skills.toggle_not_applied_impact")
      : committed
        ? t("state.committed_refresh_impact")
        : t("agent_options.skills.toggle_unknown_impact"),
    message: failure.message,
    nextStep: notApplied
      ? t("agent_options.skills.toggle_not_applied_next_step")
      : committed
        ? t("state.committed_refresh_next_step")
        : t("agent_options.skills.toggle_unknown_next_step"),
    target,
    title: notApplied
      ? t("agent_options.skills.toggle_failed")
      : committed
        ? t("agent_options.skills.toggle_committed_title")
        : t("agent_options.skills.toggle_unknown_title"),
  };
}

export function buildAgentSkillRefreshAfterMutationFailure(
  error: unknown,
  target: AgentSkillMutationTarget,
  t: I18nContextValue["t"],
): AgentSkillMutationFailure {
  return {
    blocksRepeat: true,
    effect: "committed",
    impact: t("state.committed_refresh_impact"),
    message: getErrorMessage(
      error,
      t("agent_options.skills.refresh_after_toggle_failed"),
    ),
    nextStep: t("state.committed_refresh_next_step"),
    target,
    title: t("agent_options.skills.toggle_committed_title"),
  };
}

type AvailableSkillsEmptyState =
  | "catalog_empty"
  | "no_available"
  | "no_search_match"
  | null;

export interface AgentSkillsProjection {
  available: AgentSkillEntry[];
  availableEmptyState: AvailableSkillsEmptyState;
  enabled: AgentSkillEntry[];
  visibleAvailable: AgentSkillEntry[];
}

type SkillDescriptionResolver = (skill: AgentSkillEntry) => string;

const SEARCH_FIELDS: Array<keyof Pick<
  AgentSkillEntry,
  "category_name" | "name" | "title"
>> = ["name", "title", "category_name"];

function matchesSearch(
  skill: AgentSkillEntry,
  query: string,
  resolveDescription: SkillDescriptionResolver,
): boolean {
  if (SEARCH_FIELDS.some((field) => skill[field].toLowerCase().includes(query))) {
    return true;
  }
  if (resolveDescription(skill).toLowerCase().includes(query)) {
    return true;
  }
  return skill.tags.some((tag) => tag.toLowerCase().includes(query));
}

function resolveAvailableEmptyState(
  totalCount: number,
  availableCount: number,
  visibleCount: number,
): AvailableSkillsEmptyState {
  if (visibleCount > 0) {
    return null;
  }
  const candidates = [
    { matches: totalCount === 0, state: "catalog_empty" as const },
    { matches: availableCount === 0, state: "no_available" as const },
    { matches: true, state: "no_search_match" as const },
  ];
  return candidates.find((candidate) => candidate.matches)?.state ?? null;
}

export function projectAgentSkills(
  skills: AgentSkillEntry[],
  searchQuery: string,
  resolveDescription: SkillDescriptionResolver = (skill) => skill.description,
): AgentSkillsProjection {
  const enabled: AgentSkillEntry[] = [];
  const available: AgentSkillEntry[] = [];

  for (const skill of skills) {
    if (skill.enabled_for_agent) {
      enabled.push(skill);
    } else if (!skill.locked) {
      available.push(skill);
    }
  }

  const query = searchQuery.trim().toLowerCase();
  const visibleAvailable = query
    ? available.filter((skill) => (
      matchesSearch(skill, query, resolveDescription)
    ))
    : available;
  const availableEmptyState = resolveAvailableEmptyState(
    skills.length,
    available.length,
    visibleAvailable.length,
  );

  return {
    available,
    availableEmptyState,
    enabled,
    visibleAvailable,
  };
}
