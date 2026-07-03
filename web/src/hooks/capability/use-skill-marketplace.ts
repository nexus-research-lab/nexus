import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  deleteSkillApi,
  getExternalSkillPreviewApi,
  getAvailableSkillsApi,
  importExternalSkillApi,
  importGitSkillApi,
  importLocalSkillApi,
  listExternalSkillSourcesApi,
  searchExternalSkillsApi,
  updateExternalSkillSourceApi,
  updateImportedSkillsApi,
  updateSingleSkillApi,
} from "@/lib/api/skill-api";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
  SkillActionFailure,
  SkillInfo,
} from "@/types/capability/skill";
import type {
  DiscoveryMode,
  SkillImportDialogMode,
  SkillMarketplaceController,
} from "@/features/capability/skills/skills-view-model";

const MIN_EXTERNAL_SEARCH_LENGTH = 2;

export function useSkillMarketplace(): SkillMarketplaceController {
  const [skills, setSkills] = useState<SkillInfo[]>([]);
  const [searchQuery, setSearchQuery] = useState("");
  const [debouncedSearchQuery, setDebouncedSearchQuery] = useState("");
  const [discoveryMode, setDiscoveryMode] = useState<DiscoveryMode>("catalog");
  const [activeCategory, setActiveCategory] = useState<string>("all");
  const [externalQuery, setExternalQuery] = useState("");
  const [externalSubmittedQuery, setExternalSubmittedQuery] = useState("");
  const [externalSearchRevision, setExternalSearchRevision] = useState(0);
  const [externalResults, setExternalResults] = useState<ExternalSkillSearchItem[]>([]);
  const [externalSourceStatuses, setExternalSourceStatuses] = useState<ExternalSkillSourceStatus[]>([]);
  const [externalSources, setExternalSources] = useState<ExternalSkillSourceInfo[]>([]);
  const [previewExternalItem, setPreviewExternalItem] = useState<ExternalSkillSearchItem | null>(null);
  const [externalLoading, setExternalLoading] = useState(false);
  const [externalPreviewLoading, setExternalPreviewLoading] = useState(false);
  const [sourceManagerOpen, setSourceManagerOpen] = useState(false);
  const [sourceLoading, setSourceLoading] = useState(false);
  const [sourceRevision, setSourceRevision] = useState(0);
  const [busyExternalKey, setBusyExternalKey] = useState<string | null>(null);
  const [importDialogMode, setImportDialogMode] = useState<SkillImportDialogMode | null>(null);
  const [loading, setLoading] = useState(true);
  const [busySkillName, setBusySkillName] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const externalSearchRequestRef = useRef(0);
  const externalSearchAbortRef = useRef<AbortController | null>(null);

  /* ── 数据加载 ───────────────────────────────── */

  const loadSkills = useCallback(async (query: string) => {
    const nextSkills = await getAvailableSkillsApi({
      q: query || undefined,
    });
    setSkills(nextSkills);
  }, []);

  const refreshExternalSources = useCallback(async () => {
    try {
      setSourceLoading(true);
      setErrorMessage(null);
      const nextSources = await listExternalSkillSourcesApi();
      setExternalSources(nextSources);
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "来源加载失败");
    } finally {
      setSourceLoading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      setDebouncedSearchQuery(searchQuery);
    }, 250);
    return () => {
      window.clearTimeout(timer);
    };
  }, [searchQuery]);

  useEffect(() => {
    if (discoveryMode !== "catalog") return;
    void (async () => {
      try {
        setLoading(true);
        setErrorMessage(null);
        await loadSkills(debouncedSearchQuery);
      } catch (err) {
        setErrorMessage(err instanceof Error ? err.message : "加载失败");
      } finally {
        setLoading(false);
      }
    })();
  }, [debouncedSearchQuery, discoveryMode, loadSkills]);

  useEffect(() => {
    if (discoveryMode !== "external") return;
    void refreshExternalSources();
  }, [discoveryMode, refreshExternalSources]);

  useEffect(() => {
    if (!sourceManagerOpen) return;
    void refreshExternalSources();
  }, [sourceManagerOpen, refreshExternalSources]);

  useEffect(() => {
    if (discoveryMode !== "external") return;
    if (externalQuery.trim().length >= MIN_EXTERNAL_SEARCH_LENGTH) return;
    externalSearchAbortRef.current?.abort();
    externalSearchRequestRef.current += 1;
    setExternalSubmittedQuery("");
    setExternalLoading(false);
    setExternalResults([]);
    setExternalSourceStatuses([]);
    setErrorMessage(null);
  }, [discoveryMode, externalQuery]);

  useEffect(() => {
    if (discoveryMode !== "external") return;

    const query = externalSubmittedQuery.trim();
    const requestId = ++externalSearchRequestRef.current;

    if (!query || query.length < MIN_EXTERNAL_SEARCH_LENGTH) {
      externalSearchAbortRef.current?.abort();
      externalSearchAbortRef.current = null;
      setExternalLoading(false);
      setExternalResults([]);
      setExternalSourceStatuses([]);
      setErrorMessage(null);
      return;
    }

    externalSearchAbortRef.current?.abort();
    const abortController = new AbortController();
    externalSearchAbortRef.current = abortController;
    void (async () => {
      try {
        setExternalLoading(true);
        setErrorMessage(null);
        const response = await searchExternalSkillsApi(query, false, abortController.signal);
        if (requestId !== externalSearchRequestRef.current) return;
        setExternalResults(response.results);
        setExternalSourceStatuses(response.sources);
      } catch (err) {
        if (abortController.signal.aborted) return;
        if (requestId !== externalSearchRequestRef.current) return;
        setExternalSourceStatuses([]);
        setErrorMessage(err instanceof Error ? err.message : "搜索失败");
      } finally {
        if (externalSearchAbortRef.current === abortController) {
          externalSearchAbortRef.current = null;
        }
        if (requestId === externalSearchRequestRef.current) {
          setExternalLoading(false);
        }
      }
    })();

    return () => {
      externalSearchAbortRef.current?.abort();
    };
  }, [discoveryMode, externalSearchRevision, externalSubmittedQuery, sourceRevision]);

  /* ── 派生数据 ───────────────────────────────── */

  const categories = useMemo(() => {
    const map = new Map<string, string>();
    skills.forEach((s) => map.set(s.category_key, s.category_name));
    return [{ key: "all", label: "全部" }].concat(
      Array.from(map.entries()).map(([key, label]) => ({ key, label })),
    );
  }, [skills]);

  const visibleSkills = useMemo(() => {
    let list = skills;
    if (activeCategory !== "all") {
      list = list.filter((s) => s.category_key === activeCategory);
    }
    return list;
  }, [activeCategory, skills]);

  const groupedSkills = useMemo(() => {
    const map = new Map<string, SkillInfo[]>();
    visibleSkills.forEach((s) => {
      const list = map.get(s.category_name) ?? [];
      list.push(s);
      map.set(s.category_name, list);
    });
    return Array.from(map.entries());
  }, [visibleSkills]);

  const catalogCount = skills.length;

  const importedExternalSources = useMemo(() => {
    const map = new Map<string, Set<string>>();
    skills.forEach((s) => {
      if (s.source_type !== "external") return;
      const key = s.name;
      const set = map.get(key) ?? new Set<string>();
      if (s.source_ref) set.add(s.source_ref);
      map.set(key, set);
    });
    return map;
  }, [skills]);

  /* ── 操作 ───────────────────────────────────── */

  const clearMessages = () => {
    setStatusMessage(null);
    setErrorMessage(null);
  };

  const refreshMarketplace = useCallback(async () => {
    await loadSkills(searchQuery);
  }, [loadSkills, searchQuery]);

  const submitExternalSearch = useCallback(() => {
    const query = externalQuery.trim();
    if (!query || query.length < MIN_EXTERNAL_SEARCH_LENGTH) {
      externalSearchAbortRef.current?.abort();
      externalSearchRequestRef.current += 1;
      setExternalSubmittedQuery("");
      setExternalLoading(false);
      setExternalResults([]);
      setExternalSourceStatuses([]);
      setErrorMessage(null);
      return;
    }
    setExternalSubmittedQuery(query);
    setExternalSearchRevision((value) => value + 1);
  }, [externalQuery]);

  const handleUpdateSingle = useCallback(async (skillName: string) => {
    clearMessages();
    try {
      setBusySkillName(skillName);
      await updateSingleSkillApi(skillName);
      setStatusMessage(`已更新 ${skillName}`);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "更新失败");
    } finally {
      setBusySkillName(null);
    }
  }, [refreshMarketplace]);

  const handleDeleteSkill = useCallback(async (skill: SkillInfo) => {
    clearMessages();
    try {
      setBusySkillName(skill.name);
      await deleteSkillApi(skill.name);
      setStatusMessage(`${skill.title || skill.name} 已从技能库删除`);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "删除失败");
    } finally {
      setBusySkillName(null);
    }
  }, [refreshMarketplace]);

  const handleUpdateInstalled = useCallback(async () => {
    clearMessages();
    try {
      const result = await updateImportedSkillsApi();
      setStatusMessage(
        `更新完成：更新 ${result.updated_skills.length} 个，跳过 ${result.skipped_skills.length} 个`,
      );
      if (result.failures.length) {
        setErrorMessage(
          result.failures.map((i: SkillActionFailure) => `${i.skill_name}: ${i.error}`).join("；"),
        );
      }
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "更新失败");
    }
  }, [refreshMarketplace]);

  const handleLocalImport = useCallback(async (file: File) => {
    clearMessages();
    try {
      await importLocalSkillApi(file);
      setStatusMessage(`已导入：${file.name}`);
      setImportDialogMode(null);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "导入失败");
    }
  }, [refreshMarketplace]);

  const handleGitImport = useCallback(async (url: string, branch?: string, path?: string) => {
    clearMessages();
    if (!url.trim()) return;
    try {
      await importGitSkillApi(url.trim(), branch?.trim() || undefined, path?.trim() || undefined);
      setStatusMessage("已通过 Git 导入");
      setImportDialogMode(null);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "Git 导入失败");
    }
  }, [refreshMarketplace]);

  const handlePreviewExternal = useCallback(async (item: ExternalSkillSearchItem) => {
    setPreviewExternalItem(item);
    if (item.source_kind === "skills_sh" || item.import_mode === "skills_sh") {
      return;
    }
    const previewUrl = item.raw_url || item.detail_url;
    if (item.readme_markdown || !previewUrl) {
      return;
    }
    try {
      setExternalPreviewLoading(true);
      const result = await getExternalSkillPreviewApi(previewUrl);
      setPreviewExternalItem((prev) => {
        if (!prev || prev.detail_url !== item.detail_url) return prev;
        return { ...prev, readme_markdown: result.readme_markdown };
      });
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "预览加载失败");
    } finally {
      setExternalPreviewLoading(false);
    }
  }, []);

  const handleImportExternal = useCallback(async (item: ExternalSkillSearchItem) => {
    clearMessages();
    const externalKey = `${item.source_key || item.package_spec}@@${item.skill_slug}`;
    try {
      setBusyExternalKey(externalKey);
      await importExternalSkillApi(item);
      setStatusMessage(`已导入：${item.skill_slug}`);
      await refreshMarketplace();
      setPreviewExternalItem(null);
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "导入失败");
    } finally {
      setBusyExternalKey(null);
    }
  }, [refreshMarketplace]);

  const handleToggleExternalSource = useCallback(async (
    source: ExternalSkillSourceInfo,
    enabled: boolean,
  ) => {
    clearMessages();
    try {
      setSourceLoading(true);
      await updateExternalSkillSourceApi(source.source_id, { enabled });
      setStatusMessage(`${source.name} 已${enabled ? "启用" : "停用"}`);
      await refreshExternalSources();
      setSourceRevision((value) => value + 1);
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "来源更新失败");
    } finally {
      setSourceLoading(false);
    }
  }, [refreshExternalSources]);

  return {
    // 状态
    skills,
    searchQuery,
    discoveryMode,
    activeCategory,
    externalQuery,
    externalSubmittedQuery,
    externalResults,
    externalSourceStatuses,
    externalSources,
    previewExternalItem,
    externalLoading,
    externalPreviewLoading,
    sourceManagerOpen,
    sourceLoading,
    importDialogMode,
    loading,
    busySkillName,
    busyExternalKey,
    statusMessage,
    errorMessage,
    fileInputRef,
    // 派生数据
    categories,
    visibleSkills,
    groupedSkills,
    catalogCount,
    importedExternalSources,
    // setter
    setSearchQuery,
    setDiscoveryMode,
    setActiveCategory,
    setExternalQuery,
    setPreviewExternalItem,
    setSourceManagerOpen,
    setImportDialogMode,
    setStatusMessage,
    setErrorMessage,
    // 操作
    refreshMarketplace,
    submitExternalSearch,
    handleUpdateSingle,
    handleDeleteSkill,
    handleUpdateInstalled,
    handleLocalImport,
    handleGitImport,
    handlePreviewExternal,
    handleImportExternal,
    refreshExternalSources,
    handleToggleExternalSource,
  };
}
