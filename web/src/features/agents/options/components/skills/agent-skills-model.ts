// INPUT: Agent Skill 列表、搜索条件，以及 exact Agent+Skill 读取/修改失败事实。
// OUTPUT: 列表分组投影与不猜测 mutation 结果的可见恢复模型。
// POS: Agent Options Skill 子域的纯模型；不发请求、不自动重放开关。
import {
  projectMutationFailure,
  type MutationFailureEffect,
} from "@/lib/error-message";
import type { I18nContextValue } from "@/shared/i18n/i18n-context";
import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import type { AgentSkillEntry } from "@/types/capability/skill";

export interface AgentSkillMutationTarget {
  agentId: string;
  desiredEnabled: boolean;
  skillName: string;
}

export interface AgentSkillsReadFailure {
  impact: string;
  title: string;
}

export interface AgentSkillMutationFailure {
  blocksRepeat: boolean;
  effect: MutationFailureEffect;
  impact: string;
  target: AgentSkillMutationTarget;
  title: string;
}

export function buildAgentSkillsReadFailure(
  _error: unknown,
  t: I18nContextValue["t"],
): AgentSkillsReadFailure {
  return {
    impact: t("agent_options.skills.load_failed_impact"),
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
    target,
    title: notApplied
      ? t("agent_options.skills.toggle_failed")
      : committed
        ? t("agent_options.skills.toggle_committed_title")
        : t("agent_options.skills.toggle_unknown_title"),
  };
}

export function buildAgentSkillRefreshAfterMutationFailure(
  _error: unknown,
  target: AgentSkillMutationTarget,
  t: I18nContextValue["t"],
): AgentSkillMutationFailure {
  return {
    blocksRepeat: true,
    effect: "committed",
    impact: t("state.committed_refresh_impact"),
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

  const search = createUiSearchMatcher(searchQuery);
  const visibleAvailable = available.filter((skill) => search.matches([
    skill.name,
    skill.title,
    skill.category_name,
    resolveDescription(skill),
    ...skill.tags,
  ]));
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
