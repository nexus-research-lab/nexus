import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  delete_skill_api,
  get_external_skill_preview_api,
  get_available_skills_api,
  import_external_skill_api,
  import_git_skill_api,
  import_local_skill_api,
  list_external_skill_sources_api,
  search_external_skills_api,
  update_external_skill_source_api,
  update_imported_skills_api,
  update_single_skill_api,
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
    const nextSkills = await get_available_skills_api({
      q: query || undefined,
    });
    setSkills(nextSkills);
  }, []);

  const refreshExternalSources = useCallback(async () => {
    try {
      setSourceLoading(true);
      setErrorMessage(null);
      const nextSources = await list_external_skill_sources_api();
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
        const response = await search_external_skills_api(query, false, abortController.signal);
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
      await update_single_skill_api(skillName);
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
      await delete_skill_api(skill.name);
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
      const result = await update_imported_skills_api();
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
      await import_local_skill_api(file);
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
      await import_git_skill_api(url.trim(), branch?.trim() || undefined, path?.trim() || undefined);
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
      const result = await get_external_skill_preview_api(previewUrl);
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
      await import_external_skill_api(item);
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
      await update_external_skill_source_api(source.source_id, { enabled });
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
    search_query: searchQuery,
    discovery_mode: discoveryMode,
    active_category: activeCategory,
    external_query: externalQuery,
    external_submitted_query: externalSubmittedQuery,
    external_results: externalResults,
    external_source_statuses: externalSourceStatuses,
    external_sources: externalSources,
    preview_external_item: previewExternalItem,
    external_loading: externalLoading,
    external_preview_loading: externalPreviewLoading,
    source_manager_open: sourceManagerOpen,
    source_loading: sourceLoading,
    import_dialog_mode: importDialogMode,
    loading,
    busy_skill_name: busySkillName,
    busy_external_key: busyExternalKey,
    status_message: statusMessage,
    error_message: errorMessage,
    file_input_ref: fileInputRef,
    // 派生数据
    categories,
    visible_skills: visibleSkills,
    grouped_skills: groupedSkills,
    catalog_count: catalogCount,
    imported_external_sources: importedExternalSources,
    // setter
    set_search_query: setSearchQuery,
    set_discovery_mode: setDiscoveryMode,
    set_active_category: setActiveCategory,
    set_external_query: setExternalQuery,
    set_preview_external_item: setPreviewExternalItem,
    set_source_manager_open: setSourceManagerOpen,
    set_import_dialog_mode: setImportDialogMode,
    set_status_message: setStatusMessage,
    set_error_message: setErrorMessage,
    // 操作
    refresh_marketplace: refreshMarketplace,
    submit_external_search: submitExternalSearch,
    handle_update_single: handleUpdateSingle,
    handle_delete_skill: handleDeleteSkill,
    handle_update_installed: handleUpdateInstalled,
    handle_local_import: handleLocalImport,
    handle_git_import: handleGitImport,
    handle_preview_external: handlePreviewExternal,
    handle_import_external: handleImportExternal,
    refresh_external_sources: refreshExternalSources,
    handle_toggle_external_source: handleToggleExternalSource,
  };
}
