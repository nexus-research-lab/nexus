import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  checkSkillUpdatesApi,
  deleteSkillApi,
  getExternalSkillPreviewApi,
  getAvailableSkillsApi,
  importExternalSkillApi,
  importGitSkillApi,
  importLocalSkillApi,
  listExternalSkillSourcesApi,
  searchExternalSkillsApi,
  updateExternalSkillSourceApi,
  updateSingleSkillApi,
} from "@/lib/api/skill-api";
import type {
  ExternalSkillSearchItem,
  ExternalSkillSourceInfo,
  ExternalSkillSourceStatus,
  SkillInfo,
} from "@/types/capability/skill";
import type {
  DiscoveryMode,
  SkillImportDialogMode,
  SkillMarketplaceController,
} from "@/features/capability/skills/skills-view-model";
import { formatDeployFailureMessage } from "@/features/capability/skills/skill-deploy-failures";

const MIN_EXTERNAL_SEARCH_LENGTH = 2;
const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const UPDATE_CHECK_MESSAGE_TTL_MS = 5000;
const UPDATE_CHECK_STORAGE_KEY = "nexus.skill_updates.last_checked_at";

function readLastUpdateCheckTime(): number | null {
  if (typeof window === "undefined") return null;
  const value = Number(window.localStorage.getItem(UPDATE_CHECK_STORAGE_KEY));
  return Number.isFinite(value) && value > 0 ? value : null;
}

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
  const [checkingUpdates, setCheckingUpdates] = useState(false);
  const [checkUpdateMessage, setCheckUpdateMessage] = useState<string | null>(null);
  const [lastUpdateCheckedAt, setLastUpdateCheckedAt] = useState<number | null>(readLastUpdateCheckTime);
  const [importingSkill, setImportingSkill] = useState(false);
  const [busySkillName, setBusySkillName] = useState<string | null>(null);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);
  const [warningMessage, setWarningMessage] = useState<string | null>(null);
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

  const updateAvailableSkills = useMemo(() => (
    skills.filter((skill) => skill.has_update)
  ), [skills]);

  useEffect(() => {
    if (!checkUpdateMessage || checkingUpdates || updateAvailableSkills.length > 0) return;
    const timer = window.setTimeout(() => {
      setCheckUpdateMessage(null);
    }, UPDATE_CHECK_MESSAGE_TTL_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [checkUpdateMessage, checkingUpdates, updateAvailableSkills.length]);

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
    setCheckUpdateMessage(null);
    setStatusMessage(null);
    setWarningMessage(null);
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
      const detail = await updateSingleSkillApi(skillName);
      const deployFailureMessage = formatDeployFailureMessage(skillName, detail.deploy_failures);
      if (deployFailureMessage) {
        setWarningMessage(deployFailureMessage);
      } else {
        setStatusMessage(`已更新 ${skillName}`);
      }
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

  const runUpdateCheck = useCallback(async (manual: boolean) => {
    if (checkingUpdates) return;
    if (manual) clearMessages();
    try {
      setCheckingUpdates(true);
      const result = await checkSkillUpdatesApi();
      const availableCount = result.available_skills.length;
      const failureCount = result.failures.length;
      const checkedAt = Date.now();
      window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checkedAt));
      setLastUpdateCheckedAt(checkedAt);
      const failureLabel = failureCount === 1
        ? `${result.failures[0]?.skill_name || "1 个来源"}无法检查`
        : `${failureCount} 个来源无法检查`;
      setCheckUpdateMessage(
        availableCount > 0 && failureCount > 0
          ? `发现 ${availableCount} 个可更新，${failureLabel}`
          : availableCount > 0
            ? `发现 ${availableCount} 个可更新`
            : failureCount > 0
              ? `暂无可更新，${failureLabel}`
              : manual ? "暂无更新" : null,
      );
      await refreshMarketplace();
    } catch (err) {
      if (manual) {
        setErrorMessage(err instanceof Error ? err.message : "检查失败");
      } else {
        const checkedAt = Date.now();
        window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checkedAt));
        setLastUpdateCheckedAt(checkedAt);
      }
    } finally {
      setCheckingUpdates(false);
    }
  }, [checkingUpdates, refreshMarketplace]);

  useEffect(() => {
    const now = Date.now();
    if (lastUpdateCheckedAt && now - lastUpdateCheckedAt < UPDATE_CHECK_INTERVAL_MS) return;
    void runUpdateCheck(false);
  }, [lastUpdateCheckedAt, runUpdateCheck]);

  const handleCheckUpdates = useCallback(async () => {
    await runUpdateCheck(true);
  }, [runUpdateCheck]);

  const handleLocalImport = useCallback(async (file: File) => {
    clearMessages();
    try {
      setImportingSkill(true);
      setStatusMessage(`正在导入：${file.name}...`);
      await importLocalSkillApi(file);
      setStatusMessage(`已导入：${file.name}`);
      setImportDialogMode(null);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "导入失败");
    } finally {
      setImportingSkill(false);
    }
  }, [refreshMarketplace]);

  const handleGitImport = useCallback(async (url: string, branch?: string, path?: string) => {
    clearMessages();
    if (!url.trim()) return;
    try {
      setImportingSkill(true);
      setStatusMessage("正在从 Git 拉取并导入 Skill...");
      await importGitSkillApi(url.trim(), branch?.trim() || undefined, path?.trim() || undefined);
      setStatusMessage("已通过 Git 导入");
      setImportDialogMode(null);
      await refreshMarketplace();
    } catch (err) {
      setErrorMessage(err instanceof Error ? err.message : "Git 导入失败");
    } finally {
      setImportingSkill(false);
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
      setStatusMessage(`正在导入：${item.skill_slug}...`);
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
    checkingUpdates,
    checkUpdateMessage,
    lastUpdateCheckedAt,
    importingSkill,
    busySkillName,
    busyExternalKey,
    statusMessage,
    warningMessage,
    errorMessage,
    fileInputRef,
    // 派生数据
    categories,
    visibleSkills,
    updateAvailableSkills,
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
    setWarningMessage,
    setErrorMessage,
    // 操作
    refreshMarketplace,
    submitExternalSearch,
    handleUpdateSingle,
    handleDeleteSkill,
    handleCheckUpdates,
    handleLocalImport,
    handleGitImport,
    handlePreviewExternal,
    handleImportExternal,
    refreshExternalSources,
    handleToggleExternalSource,
  };
}
