import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  check_skill_updates_api,
  delete_skill_api,
  get_external_skill_preview_api,
  get_available_skills_api,
  import_external_skill_api,
  import_git_skill_api,
  import_local_skill_api,
  list_external_skill_sources_api,
  search_external_skills_api,
  update_external_skill_source_api,
  update_single_skill_api,
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

const MIN_EXTERNAL_SEARCH_LENGTH = 2;
const UPDATE_CHECK_INTERVAL_MS = 24 * 60 * 60 * 1000;
const UPDATE_CHECK_MESSAGE_TTL_MS = 5000;
const UPDATE_CHECK_STORAGE_KEY = "nexus.skill_updates.last_checked_at";

function read_last_update_check_time(): number | null {
  if (typeof window === "undefined") return null;
  const value = Number(window.localStorage.getItem(UPDATE_CHECK_STORAGE_KEY));
  return Number.isFinite(value) && value > 0 ? value : null;
}

export function useSkillMarketplace(): SkillMarketplaceController {
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [search_query, set_search_query] = useState("");
  const [debounced_search_query, set_debounced_search_query] = useState("");
  const [discovery_mode, set_discovery_mode] = useState<DiscoveryMode>("catalog");
  const [active_category, set_active_category] = useState<string>("all");
  const [external_query, set_external_query] = useState("");
  const [external_submitted_query, set_external_submitted_query] = useState("");
  const [external_search_revision, set_external_search_revision] = useState(0);
  const [external_results, set_external_results] = useState<ExternalSkillSearchItem[]>([]);
  const [external_source_statuses, set_external_source_statuses] = useState<ExternalSkillSourceStatus[]>([]);
  const [external_sources, set_external_sources] = useState<ExternalSkillSourceInfo[]>([]);
  const [preview_external_item, set_preview_external_item] = useState<ExternalSkillSearchItem | null>(null);
  const [external_loading, set_external_loading] = useState(false);
  const [external_preview_loading, set_external_preview_loading] = useState(false);
  const [source_manager_open, set_source_manager_open] = useState(false);
  const [source_loading, set_source_loading] = useState(false);
  const [source_revision, set_source_revision] = useState(0);
  const [busy_external_key, set_busy_external_key] = useState<string | null>(null);
  const [import_dialog_mode, set_import_dialog_mode] = useState<SkillImportDialogMode | null>(null);
  const [loading, set_loading] = useState(true);
  const [checking_updates, set_checking_updates] = useState(false);
  const [check_update_message, set_check_update_message] = useState<string | null>(null);
  const [last_update_checked_at, set_last_update_checked_at] = useState<number | null>(read_last_update_check_time);
  const [importing_skill, set_importing_skill] = useState(false);
  const [busy_skill_name, set_busy_skill_name] = useState<string | null>(null);
  const [status_message, set_status_message] = useState<string | null>(null);
  const [error_message, set_error_message] = useState<string | null>(null);
  const file_input_ref = useRef<HTMLInputElement | null>(null);
  const external_search_request_ref = useRef(0);
  const external_search_abort_ref = useRef<AbortController | null>(null);

  /* ── 数据加载 ───────────────────────────────── */

  const load_skills = useCallback(async (query: string) => {
    const next_skills = await get_available_skills_api({
      q: query || undefined,
    });
    set_skills(next_skills);
  }, []);

  const refresh_external_sources = useCallback(async () => {
    try {
      set_source_loading(true);
      set_error_message(null);
      const next_sources = await list_external_skill_sources_api();
      set_external_sources(next_sources);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "来源加载失败");
    } finally {
      set_source_loading(false);
    }
  }, []);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      set_debounced_search_query(search_query);
    }, 250);
    return () => {
      window.clearTimeout(timer);
    };
  }, [search_query]);

  useEffect(() => {
    if (discovery_mode !== "catalog") return;
    void (async () => {
      try {
        set_loading(true);
        set_error_message(null);
        await load_skills(debounced_search_query);
      } catch (err) {
        set_error_message(err instanceof Error ? err.message : "加载失败");
      } finally {
        set_loading(false);
      }
    })();
  }, [debounced_search_query, discovery_mode, load_skills]);

  useEffect(() => {
    if (discovery_mode !== "external") return;
    void refresh_external_sources();
  }, [discovery_mode, refresh_external_sources]);

  useEffect(() => {
    if (!source_manager_open) return;
    void refresh_external_sources();
  }, [source_manager_open, refresh_external_sources]);

  useEffect(() => {
    if (discovery_mode !== "external") return;
    if (external_query.trim().length >= MIN_EXTERNAL_SEARCH_LENGTH) return;
    external_search_abort_ref.current?.abort();
    external_search_request_ref.current += 1;
    set_external_submitted_query("");
    set_external_loading(false);
    set_external_results([]);
    set_external_source_statuses([]);
    set_error_message(null);
  }, [discovery_mode, external_query]);

  useEffect(() => {
    if (discovery_mode !== "external") return;

    const query = external_submitted_query.trim();
    const request_id = ++external_search_request_ref.current;

    if (!query || query.length < MIN_EXTERNAL_SEARCH_LENGTH) {
      external_search_abort_ref.current?.abort();
      external_search_abort_ref.current = null;
      set_external_loading(false);
      set_external_results([]);
      set_external_source_statuses([]);
      set_error_message(null);
      return;
    }

    external_search_abort_ref.current?.abort();
    const abort_controller = new AbortController();
    external_search_abort_ref.current = abort_controller;
    void (async () => {
      try {
        set_external_loading(true);
        set_error_message(null);
        const response = await search_external_skills_api(query, false, abort_controller.signal);
        if (request_id !== external_search_request_ref.current) return;
        set_external_results(response.results);
        set_external_source_statuses(response.sources);
      } catch (err) {
        if (abort_controller.signal.aborted) return;
        if (request_id !== external_search_request_ref.current) return;
        set_external_source_statuses([]);
        set_error_message(err instanceof Error ? err.message : "搜索失败");
      } finally {
        if (external_search_abort_ref.current === abort_controller) {
          external_search_abort_ref.current = null;
        }
        if (request_id === external_search_request_ref.current) {
          set_external_loading(false);
        }
      }
    })();

    return () => {
      external_search_abort_ref.current?.abort();
    };
  }, [discovery_mode, external_search_revision, external_submitted_query, source_revision]);

  /* ── 派生数据 ───────────────────────────────── */

  const categories = useMemo(() => {
    const map = new Map<string, string>();
    skills.forEach((s) => map.set(s.category_key, s.category_name));
    return [{ key: "all", label: "全部" }].concat(
      Array.from(map.entries()).map(([key, label]) => ({ key, label })),
    );
  }, [skills]);

  const visible_skills = useMemo(() => {
    let list = skills;
    if (active_category !== "all") {
      list = list.filter((s) => s.category_key === active_category);
    }
    return list;
  }, [active_category, skills]);

  const update_available_skills = useMemo(() => (
    skills.filter((skill) => skill.has_update)
  ), [skills]);

  useEffect(() => {
    if (!check_update_message || checking_updates || update_available_skills.length > 0) return;
    const timer = window.setTimeout(() => {
      set_check_update_message(null);
    }, UPDATE_CHECK_MESSAGE_TTL_MS);
    return () => {
      window.clearTimeout(timer);
    };
  }, [check_update_message, checking_updates, update_available_skills.length]);

  const grouped_skills = useMemo(() => {
    const map = new Map<string, SkillInfo[]>();
    visible_skills.forEach((s) => {
      const list = map.get(s.category_name) ?? [];
      list.push(s);
      map.set(s.category_name, list);
    });
    return Array.from(map.entries());
  }, [visible_skills]);

  const catalog_count = skills.length;

  const imported_external_sources = useMemo(() => {
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

  const clear_messages = () => {
    set_check_update_message(null);
    set_status_message(null);
    set_error_message(null);
  };

  const refresh_marketplace = useCallback(async () => {
    await load_skills(search_query);
  }, [load_skills, search_query]);

  const submit_external_search = useCallback(() => {
    const query = external_query.trim();
    if (!query || query.length < MIN_EXTERNAL_SEARCH_LENGTH) {
      external_search_abort_ref.current?.abort();
      external_search_request_ref.current += 1;
      set_external_submitted_query("");
      set_external_loading(false);
      set_external_results([]);
      set_external_source_statuses([]);
      set_error_message(null);
      return;
    }
    set_external_submitted_query(query);
    set_external_search_revision((value) => value + 1);
  }, [external_query]);

  const handle_update_single = useCallback(async (skill_name: string) => {
    clear_messages();
    try {
      set_busy_skill_name(skill_name);
      await update_single_skill_api(skill_name);
      set_status_message(`已更新 ${skill_name}`);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "更新失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace]);

  const handle_delete_skill = useCallback(async (skill: SkillInfo) => {
    clear_messages();
    try {
      set_busy_skill_name(skill.name);
      await delete_skill_api(skill.name);
      set_status_message(`${skill.title || skill.name} 已从技能库删除`);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "删除失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace]);

  const run_update_check = useCallback(async (manual: boolean) => {
    if (checking_updates) return;
    if (manual) clear_messages();
    try {
      set_checking_updates(true);
      const result = await check_skill_updates_api();
      const available_count = result.available_skills.length;
      const failure_count = result.failures.length;
      const checked_at = Date.now();
      window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checked_at));
      set_last_update_checked_at(checked_at);
      const failure_label = failure_count === 1
        ? `${result.failures[0]?.skill_name || "1 个来源"}无法检查`
        : `${failure_count} 个来源无法检查`;
      set_check_update_message(
        available_count > 0 && failure_count > 0
          ? `发现 ${available_count} 个可更新，${failure_label}`
          : available_count > 0
            ? `发现 ${available_count} 个可更新`
            : failure_count > 0
              ? `暂无可更新，${failure_label}`
              : manual ? "暂无更新" : null,
      );
      await refresh_marketplace();
    } catch (err) {
      if (manual) {
        set_error_message(err instanceof Error ? err.message : "检查失败");
      } else {
        const checked_at = Date.now();
        window.localStorage.setItem(UPDATE_CHECK_STORAGE_KEY, String(checked_at));
        set_last_update_checked_at(checked_at);
      }
    } finally {
      set_checking_updates(false);
    }
  }, [checking_updates, refresh_marketplace]);

  useEffect(() => {
    const now = Date.now();
    if (last_update_checked_at && now - last_update_checked_at < UPDATE_CHECK_INTERVAL_MS) return;
    void run_update_check(false);
  }, [last_update_checked_at, run_update_check]);

  const handle_check_updates = useCallback(async () => {
    await run_update_check(true);
  }, [run_update_check]);

  const handle_local_import = useCallback(async (file: File) => {
    clear_messages();
    try {
      set_importing_skill(true);
      set_status_message(`正在导入：${file.name}...`);
      await import_local_skill_api(file);
      set_status_message(`已导入：${file.name}`);
      set_import_dialog_mode(null);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "导入失败");
    } finally {
      set_importing_skill(false);
    }
  }, [refresh_marketplace]);

  const handle_git_import = useCallback(async (url: string, branch?: string, path?: string) => {
    clear_messages();
    if (!url.trim()) return;
    try {
      set_importing_skill(true);
      set_status_message("正在从 Git 拉取并导入 Skill...");
      await import_git_skill_api(url.trim(), branch?.trim() || undefined, path?.trim() || undefined);
      set_status_message("已通过 Git 导入");
      set_import_dialog_mode(null);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "Git 导入失败");
    } finally {
      set_importing_skill(false);
    }
  }, [refresh_marketplace]);

  const handle_preview_external = useCallback(async (item: ExternalSkillSearchItem) => {
    set_preview_external_item(item);
    if (item.source_kind === "skills_sh" || item.import_mode === "skills_sh") {
      return;
    }
    const preview_url = item.raw_url || item.detail_url;
    if (item.readme_markdown || !preview_url) {
      return;
    }
    try {
      set_external_preview_loading(true);
      const result = await get_external_skill_preview_api(preview_url);
      set_preview_external_item((prev) => {
        if (!prev || prev.detail_url !== item.detail_url) return prev;
        return { ...prev, readme_markdown: result.readme_markdown };
      });
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "预览加载失败");
    } finally {
      set_external_preview_loading(false);
    }
  }, []);

  const handle_import_external = useCallback(async (item: ExternalSkillSearchItem) => {
    clear_messages();
    const external_key = `${item.source_key || item.package_spec}@@${item.skill_slug}`;
    try {
      set_busy_external_key(external_key);
      set_status_message(`正在导入：${item.skill_slug}...`);
      await import_external_skill_api(item);
      set_status_message(`已导入：${item.skill_slug}`);
      await refresh_marketplace();
      set_preview_external_item(null);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "导入失败");
    } finally {
      set_busy_external_key(null);
    }
  }, [refresh_marketplace]);

  const handle_toggle_external_source = useCallback(async (
    source: ExternalSkillSourceInfo,
    enabled: boolean,
  ) => {
    clear_messages();
    try {
      set_source_loading(true);
      await update_external_skill_source_api(source.source_id, { enabled });
      set_status_message(`${source.name} 已${enabled ? "启用" : "停用"}`);
      await refresh_external_sources();
      set_source_revision((value) => value + 1);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "来源更新失败");
    } finally {
      set_source_loading(false);
    }
  }, [refresh_external_sources]);

  return {
    // 状态
    skills,
    search_query,
    discovery_mode,
    active_category,
    external_query,
    external_submitted_query,
    external_results,
    external_source_statuses,
    external_sources,
    preview_external_item,
    external_loading,
    external_preview_loading,
    source_manager_open,
    source_loading,
    import_dialog_mode,
    loading,
    checking_updates,
    check_update_message,
    last_update_checked_at,
    importing_skill,
    busy_skill_name,
    busy_external_key,
    status_message,
    error_message,
    file_input_ref,
    // 派生数据
    categories,
    visible_skills,
    update_available_skills,
    grouped_skills,
    catalog_count,
    imported_external_sources,
    // setter
    set_search_query,
    set_discovery_mode,
    set_active_category,
    set_external_query,
    set_preview_external_item,
    set_source_manager_open,
    set_import_dialog_mode,
    set_status_message,
    set_error_message,
    // 操作
    refresh_marketplace,
    submit_external_search,
    handle_update_single,
    handle_delete_skill,
    handle_check_updates,
    handle_local_import,
    handle_git_import,
    handle_preview_external,
    handle_import_external,
    refresh_external_sources,
    handle_toggle_external_source,
  };
}
