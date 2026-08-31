// INPUT: 当前发现模式、搜索条件与目录失败反馈入口。
// OUTPUT: 只提交最新请求的 Skill 目录快照，以及可判定成功/失败的只读刷新结果。
// POS: Skill marketplace 目录资源边界；刷新不执行或重放任何写操作。
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAvailableSkillsApi } from "@/lib/api/capability/skill-api";
import { getSkillCategoryLabel } from "@/lib/skill-category";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { SkillInfo } from "@/types/capability/skill";

import type { SkillCatalogController } from "./skill-marketplace-controller";

const SEARCH_DEBOUNCE_MS = 250;

interface UseSkillCatalogOptions {
  active: boolean;
  onError: (message: string) => void;
}

export function useSkillCatalog({
  active,
  onError,
}: UseSkillCatalogOptions): SkillCatalogController {
  const { t } = useI18n();
  const catalogLoadFailed = t("capability.skills_catalog_load_failed");
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [query, setQuery] = useState("");
  const [debouncedQuery, setDebouncedQuery] = useState("");
  const [activeCategory, setActiveCategory] = useState("all");
  const [loading, setLoading] = useState(true);
  const requestRef = useRef(0);

  useEffect(() => {
    const timer = window.setTimeout(
      () => setDebouncedQuery(query),
      SEARCH_DEBOUNCE_MS,
    );
    return () => window.clearTimeout(timer);
  }, [query]);

  const load = useCallback(async (searchQuery: string) => {
    const requestId = ++requestRef.current;
    setLoading(true);
    try {
      const nextSkills = await getAvailableSkillsApi({
        q: searchQuery || undefined,
      });
      if (requestId === requestRef.current) {
        setSkills(nextSkills);
        return true;
      }
      return false;
    } catch {
      if (requestId === requestRef.current) {
        onError(catalogLoadFailed);
      }
      return false;
    } finally {
      if (requestId === requestRef.current) {
        setLoading(false);
      }
    }
  }, [catalogLoadFailed, onError]);

  useEffect(() => {
    if (!active) return;
    void load(debouncedQuery);
  }, [active, debouncedQuery, load]);

  const categories = useMemo(() => {
    const categoryNames = new Map<string, string>();
    skills.forEach((skill) => {
      categoryNames.set(
        skill.category_key,
        getSkillCategoryLabel(skill, t),
      );
    });
    return [
      { key: "all", label: t("capability.category_all") },
      ...Array.from(categoryNames, ([key, label]) => ({ key, label })),
    ];
  }, [skills, t]);
  const availableCategoryKeys = useMemo(
    () => new Set(categories.map((category) => category.key)),
    [categories],
  );
  const selectedCategory = availableCategoryKeys.has(activeCategory)
    ? activeCategory
    : "all";
  const visibleSkills = useMemo(
    () => selectedCategory === "all"
      ? skills
      : skills.filter((skill) => skill.category_key === selectedCategory),
    [selectedCategory, skills],
  );
  const groupedSkills = useMemo(() => {
    const groups = new Map<string, [string, SkillInfo[]]>();
    visibleSkills.forEach((skill) => {
      const group = groups.get(skill.category_key) ?? [
        getSkillCategoryLabel(skill, t),
        [],
      ];
      group[1].push(skill);
      groups.set(skill.category_key, group);
    });
    return Array.from(groups.values());
  }, [t, visibleSkills]);
  const updateAvailableSkills = useMemo(
    () => skills.filter((skill) => skill.has_update),
    [skills],
  );
  const importedExternalSources = useMemo(() => {
    const sources = new Map<string, Set<string>>();
    skills.forEach((skill) => {
      if (skill.source_type !== "external") return;
      const refs = sources.get(skill.name) ?? new Set<string>();
      if (skill.source_ref) refs.add(skill.source_ref);
      sources.set(skill.name, refs);
    });
    return sources;
  }, [skills]);
  const refresh = useCallback(() => load(query), [load, query]);

  return {
    activeCategory: selectedCategory,
    categories,
    groupedSkills,
    importedExternalSources,
    loading,
    query,
    refresh,
    setActiveCategory,
    setQuery,
    skills,
    updateAvailableSkills,
  };
}
