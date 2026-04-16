import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import {
  delete_skill_api,
  get_external_skill_preview_api,
  get_available_skills_api,
  import_skills_sh_skill_api,
  import_git_skill_api,
  import_local_skill_api,
  search_external_skills_api,
  update_imported_skills_api,
  update_single_skill_api,
} from "@/lib/api/skill-api";
import type { ExternalSkillSearchItem, SkillActionFailure, SkillInfo } from "@/types/capability/skill";

export type DiscoveryMode = "catalog" | "external";

export function format_installs(n: number) {
  return n >= 1000 ? `${(n / 1000).toFixed(n >= 100000 ? 0 : 1)}K` : `${n}`;
}

export function useSkillMarketplace() {
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [search_query, set_search_query] = useState("");
  const [debounced_search_query, set_debounced_search_query] = useState("");
  const [discovery_mode, set_discovery_mode] = useState<DiscoveryMode>("catalog");
  const [active_category, set_active_category] = useState<string>("all");
  const [selected_skill, set_selected_skill] = useState<string | null>(null);
  const [external_query, set_external_query] = useState("");
  const [external_results, set_external_results] = useState<ExternalSkillSearchItem[]>([]);
  const [preview_external_item, set_preview_external_item] = useState<ExternalSkillSearchItem | null>(null);
  const [external_loading, set_external_loading] = useState(false);
  const [external_preview_loading, set_external_preview_loading] = useState(false);
  const [busy_external_key, set_busy_external_key] = useState<string | null>(null);
  const [git_prompt_open, set_git_prompt_open] = useState(false);
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

    const query = external_query.trim();
    const request_id = ++external_search_request_ref.current;

    if (!query) {
      set_external_loading(false);
      set_external_results([]);
      set_error_message(null);
      return;
    }

    const timer = window.setTimeout(() => {
      void (async () => {
        try {
          set_external_loading(true);
          set_error_message(null);
          const results = await search_external_skills_api(query, false);
          if (request_id !== external_search_request_ref.current) return;
          set_external_results(results);
        } catch (err) {
          if (request_id !== external_search_request_ref.current) return;
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
  }, [discovery_mode, external_query]);

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
      if (selected_skill === skill.name) set_selected_skill(null);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "删除失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace, selected_skill]);

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
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "导入失败");
    }
  }, [refresh_marketplace]);

  const handle_git_import = useCallback(async (url: string) => {
    clear_messages();
    if (!url.trim()) return;
    try {
      await import_git_skill_api(url.trim());
      set_status_message("已通过 Git 导入");
      set_git_prompt_open(false);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "Git 导入失败");
    }
  }, [refresh_marketplace]);

  const handle_preview_external = useCallback(async (item: ExternalSkillSearchItem) => {
    set_preview_external_item(item);
    if (item.readme_markdown || !item.detail_url) {
      return;
    }
    try {
      set_external_preview_loading(true);
      const result = await get_external_skill_preview_api(item.detail_url);
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
    const external_key = `${item.package_spec}@@${item.skill_slug}`;
    try {
      set_busy_external_key(external_key);
      await import_skills_sh_skill_api(item.package_spec, item.skill_slug);
      set_status_message(`已导入：${item.skill_slug}`);
      await refresh_marketplace();
      set_preview_external_item(null);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "导入失败");
    } finally {
      set_busy_external_key(null);
    }
  }, [refresh_marketplace]);

  return {
    // 状态
    skills,
    search_query,
    discovery_mode,
    active_category,
    selected_skill,
    external_query,
    external_results,
    preview_external_item,
    external_loading,
    external_preview_loading,
    git_prompt_open,
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
    set_selected_skill,
    set_external_query,
    set_preview_external_item,
    set_git_prompt_open,
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
  };
}

export type SkillMarketplaceController = ReturnType<typeof useSkillMarketplace>;
