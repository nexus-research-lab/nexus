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

export function useSkillMarketplace(): SkillMarketplaceController {
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [search_query, set_search_query] = useState("");
  const [debounced_search_query, set_debounced_search_query] = useState("");
  const [discovery_mode, set_discovery_mode] = useState<DiscoveryMode>("catalog");
  const [active_category, set_active_category] = useState<string>("all");
  const [external_query, set_external_query] = useState("");
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
  const [busy_skill_name, set_busy_skill_name] = useState<string | null>(null);
  const [status_message, set_status_message] = useState<string | null>(null);
  const [error_message, set_error_message] = useState<string | null>(null);
  const file_input_ref = useRef<HTMLInputElement | null>(null);
  const external_search_request_ref = useRef(0);

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

    const query = external_query.trim();
    const request_id = ++external_search_request_ref.current;

    if (!query) {
      set_external_loading(false);
      set_external_results([]);
      set_external_source_statuses([]);
      set_error_message(null);
      return;
    }

    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          set_external_loading(true);
          set_error_message(null);
          const response = await search_external_skills_api(query, false);
          if (request_id !== external_search_request_ref.current) return;
          set_external_results(response.results);
          set_external_source_statuses(response.sources);
        } catch (err) {
          if (request_id !== external_search_request_ref.current) return;
          set_external_source_statuses([]);
          set_error_message(err instanceof Error ? err.message : "搜索失败");
        } finally {
          if (request_id === external_search_request_ref.current) {
            set_external_loading(false);
          }
        }
      })();
    }, 280);

    return () => {
      window.clearTimeout(timer);
    };
  }, [discovery_mode, external_query, source_revision]);

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
    set_status_message(null);
    set_error_message(null);
  };

  const refresh_marketplace = useCallback(async () => {
    await load_skills(search_query);
  }, [load_skills, search_query]);

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

  const handle_update_installed = useCallback(async () => {
    clear_messages();
    try {
      const result = await update_imported_skills_api();
      set_status_message(
        `更新完成：更新 ${result.updated_skills.length} 个，跳过 ${result.skipped_skills.length} 个`,
      );
      if (result.failures.length) {
        set_error_message(
          result.failures.map((i: SkillActionFailure) => `${i.skill_name}: ${i.error}`).join("；"),
        );
      }
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "更新失败");
    }
  }, [refresh_marketplace]);

  const handle_local_import = useCallback(async (file: File) => {
    clear_messages();
    try {
      await import_local_skill_api(file);
      set_status_message(`已导入：${file.name}`);
      set_import_dialog_mode(null);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "导入失败");
    }
  }, [refresh_marketplace]);

  const handle_git_import = useCallback(async (url: string, branch?: string, path?: string) => {
    clear_messages();
    if (!url.trim()) return;
    try {
      await import_git_skill_api(url.trim(), branch?.trim() || undefined, path?.trim() || undefined);
      set_status_message("已通过 Git 导入");
      set_import_dialog_mode(null);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "Git 导入失败");
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
    busy_skill_name,
    busy_external_key,
    status_message,
    error_message,
    file_input_ref,
    // 派生数据
    categories,
    visible_skills,
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
    handle_update_single,
    handle_delete_skill,
    handle_update_installed,
    handle_local_import,
    handle_git_import,
    handle_preview_external,
    handle_import_external,
    refresh_external_sources,
    handle_toggle_external_source,
  };
}
