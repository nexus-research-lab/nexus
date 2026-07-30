import type { AgentSkillEntry } from "@/types/capability/skill";

type AvailableSkillsEmptyState =
  | "catalog_empty"
  | "no_available"
  | "no_search_match"
  | null;

export interface AgentSkillsProjection {
  available: AgentSkillEntry[];
  availableEmptyState: AvailableSkillsEmptyState;
  enabled: AgentSkillEntry[];
  totalCount: number;
  visibleAvailable: AgentSkillEntry[];
}

const SEARCH_FIELDS: Array<keyof Pick<
  AgentSkillEntry,
  "category_name" | "description" | "name" | "title"
>> = ["name", "title", "description", "category_name"];

function matchesSearch(skill: AgentSkillEntry, query: string): boolean {
  if (SEARCH_FIELDS.some((field) => skill[field].toLowerCase().includes(query))) {
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
    ? available.filter((skill) => matchesSearch(skill, query))
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
    totalCount: skills.length,
    visibleAvailable,
  };
}
