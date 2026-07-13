import type { AgentSkillEntry } from "@/types/capability/skill";

type AvailableSkillsEmptyState =
  | "catalog_empty"
  | "no_addable"
  | "no_search_match"
  | null;

export interface AgentSkillsProjection {
  addable: AgentSkillEntry[];
  availableEmptyState: AvailableSkillsEmptyState;
  installed: AgentSkillEntry[];
  totalCount: number;
  visibleAddable: AgentSkillEntry[];
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
  addableCount: number,
  visibleCount: number,
): AvailableSkillsEmptyState {
  if (visibleCount > 0) {
    return null;
  }
  const candidates = [
    { matches: totalCount === 0, state: "catalog_empty" as const },
    { matches: addableCount === 0, state: "no_addable" as const },
    { matches: true, state: "no_search_match" as const },
  ];
  return candidates.find((candidate) => candidate.matches)?.state ?? null;
}

export function projectAgentSkills(
  skills: AgentSkillEntry[],
  searchQuery: string,
): AgentSkillsProjection {
  const installed: AgentSkillEntry[] = [];
  const addable: AgentSkillEntry[] = [];

  for (const skill of skills) {
    if (skill.installed) {
      installed.push(skill);
    } else if (!skill.locked) {
      addable.push(skill);
    }
  }

  const query = searchQuery.trim().toLowerCase();
  const visibleAddable = query
    ? addable.filter((skill) => matchesSearch(skill, query))
    : addable;
  const availableEmptyState = resolveAvailableEmptyState(
    skills.length,
    addable.length,
    visibleAddable.length,
  );

  return {
    addable,
    availableEmptyState,
    installed,
    totalCount: skills.length,
    visibleAddable,
  };
}
