import { createUiSearchMatcher } from "@/shared/ui/form/search-query";
import type { LoopCatalogItem } from "@/types/capability/loop";

export const ALL_LOOP_CATEGORIES = "__all__";

export type LoopPickerContentKind = "empty" | "error" | "list" | "loading";

export function buildLoopCategoryOptions(
  loops: LoopCatalogItem[],
  allLabel: string,
): Array<{ label: string; value: string }> {
  const categories = Array.from(
    new Set(loops.map((loop) => loop.category)),
  ).sort();
  return [
    { label: allLabel, value: ALL_LOOP_CATEGORIES },
    ...categories.map((category) => ({ label: category, value: category })),
  ];
}

export function filterLoops(
  loops: LoopCatalogItem[],
  category: string,
  query: string,
): LoopCatalogItem[] {
  const search = createUiSearchMatcher(query);
  return loops.filter((loop) => [
    matchesLoopCategory(loop, category),
    search.matches([
      loop.title,
      loop.description,
      loop.category,
      loop.trigger_type,
      ...loop.tags,
      ...loop.compatible_agents,
    ]),
  ].every(Boolean));
}

function matchesLoopCategory(
  loop: LoopCatalogItem,
  category: string,
): boolean {
  return new Set([ALL_LOOP_CATEGORIES, loop.category]).has(category);
}

export function projectLoopPickerContentKind({
  accessBlocked,
  error,
  hasSnapshot,
  isLoading,
  loopCount,
}: {
  accessBlocked: boolean;
  error: unknown | null;
  hasSnapshot: boolean;
  isLoading: boolean;
  loopCount: number;
}): LoopPickerContentKind {
  const candidates: Array<{
    active: boolean;
    kind: LoopPickerContentKind;
  }> = [
    { active: isLoading && !hasSnapshot, kind: "loading" },
    { active: Boolean(error) && (accessBlocked || !hasSnapshot), kind: "error" },
    { active: loopCount === 0, kind: "empty" },
  ];
  return candidates.find((candidate) => candidate.active)?.kind ?? "list";
}
