// INPUT: Loop 目录、分类、查询与已确认的资源状态。
// OUTPUT: 分类选项、保留原始条目的搜索结果与加载/错误/空/列表投影。
// POS: Loop picker 纯模型；不发起请求或复制公共视图样式。

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
  return loops.filter((loop) => (category === ALL_LOOP_CATEGORIES || category === loop.category)
    && search.matches([
      loop.title,
      loop.description,
      loop.category,
      loop.trigger_type,
      ...loop.tags,
      ...loop.compatible_agents,
    ]));
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
