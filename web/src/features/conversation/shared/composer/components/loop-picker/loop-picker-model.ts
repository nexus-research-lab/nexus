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
  const normalizedQuery = query.trim().toLowerCase();
  return loops.filter((loop) => [
    matchesLoopCategory(loop, category),
    buildLoopSearchText(loop).includes(normalizedQuery),
  ].every(Boolean));
}

function matchesLoopCategory(
  loop: LoopCatalogItem,
  category: string,
): boolean {
  return new Set([ALL_LOOP_CATEGORIES, loop.category]).has(category);
}

function buildLoopSearchText(loop: LoopCatalogItem): string {
  return [
    loop.title,
    loop.description,
    loop.category,
    loop.trigger_type,
    ...loop.tags,
    ...loop.compatible_agents,
  ].join(" ").toLowerCase();
}

export function projectLoopPickerContentKind({
  error,
  isLoading,
  loopCount,
}: {
  error: string | null;
  isLoading: boolean;
  loopCount: number;
}): LoopPickerContentKind {
  const candidates: Array<{
    active: boolean;
    kind: LoopPickerContentKind;
  }> = [
    { active: isLoading, kind: "loading" },
    { active: Boolean(error), kind: "error" },
    { active: loopCount === 0, kind: "empty" },
  ];
  return candidates.find((candidate) => candidate.active)?.kind ?? "list";
}
