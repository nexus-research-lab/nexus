import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAvailableSkillsApi } from "@/lib/api/skill-api";
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
      }
    } catch (error) {
      if (requestId === requestRef.current) {
        onError(error instanceof Error ? error.message : "技能目录加载失败");
      }
    } finally {
      if (requestId === requestRef.current) {
        setLoading(false);
      }
    }
  }, [onError]);

  useEffect(() => {
    if (!active) return;
    void load(debouncedQuery);
  }, [active, debouncedQuery, load]);

  const categories = useMemo(() => {
    const categoryNames = new Map<string, string>();
    skills.forEach((skill) => {
      categoryNames.set(skill.category_key, skill.category_name);
    });
    return [
      { key: "all", label: "全部" },
      ...Array.from(categoryNames, ([key, label]) => ({ key, label })),
    ];
  }, [skills]);
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
    const groups = new Map<string, SkillInfo[]>();
    visibleSkills.forEach((skill) => {
      const group = groups.get(skill.category_name) ?? [];
      group.push(skill);
      groups.set(skill.category_name, group);
    });
    return Array.from(groups);
  }, [visibleSkills]);
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
    catalogCount: skills.length,
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
