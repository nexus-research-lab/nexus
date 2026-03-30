"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  AlertCircle,
  CheckCircle2,
  Download,
  FolderUp,
  Globe2,
  Import,
  Loader2,
  Puzzle,
  RefreshCw,
  Search,
  Sparkles,
} from "lucide-react";

import {
  deleteSkillFromPoolApi,
  getAvailableSkillsApi,
  importSkillsShSkillApi,
  importGitSkillApi,
  importLocalSkillApi,
  searchExternalSkillsApi,
  setSkillGlobalEnabledApi,
  updateImportedSkillsApi,
  updateSingleSkillApi,
} from "@/lib/skill-api";
import { PromptDialog } from "@/shared/ui/confirm-dialog";
import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSearchInput } from "@/shared/ui/workspace-search-input";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";
import { ExternalSkillSearchItem, SkillInfo } from "@/types/skill";

import { SkillDetailDialog } from "./skill-detail-dialog";
import { ExternalSkillPreviewDialog } from "./external-skill-preview-dialog";
import { SkillsCard } from "./skills-card";

type SourceFilter = "all" | "builtin" | "external" | "system";
type DiscoveryMode = "catalog" | "external";

function splitFeedbackItems(message: string): string[] {
  return message
    .split(/[；\n]+/)
    .map((item) => item.trim())
    .filter(Boolean);
}

interface FeedbackBannerProps {
  tone: "success" | "error";
  title: string;
  message: string;
}

function FeedbackBanner({ tone, title, message }: FeedbackBannerProps) {
  const items = splitFeedbackItems(message);
  const is_success = tone === "success";
  const Icon = is_success ? CheckCircle2 : AlertCircle;

  return (
    <div
      className={
        "workspace-card mb-4 rounded-[24px] px-4 py-4 " +
        (is_success
          ? "border border-emerald-200/70 bg-emerald-50/75"
          : "border border-rose-200/70 bg-rose-50/75")
      }
    >
      <div className="flex items-start gap-3">
        <div
          className={
            "mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full " +
            (is_success ? "bg-emerald-100 text-emerald-600" : "bg-rose-100 text-rose-600")
          }
        >
          <Icon className="h-4 w-4" />
        </div>
        <div className="min-w-0 flex-1">
          <p
            className={
              "text-[13px] font-bold " +
              (is_success ? "text-emerald-700" : "text-rose-700")
            }
          >
            {title}
          </p>
          {items.length > 1 ? (
            <div className="mt-2 flex flex-wrap gap-2">
              {items.map((item) => (
                <span
                  key={item}
                  className={
                    "inline-flex rounded-full px-2.5 py-1 text-[11px] font-medium " +
                    (is_success
                      ? "bg-white/70 text-emerald-700"
                      : "bg-white/70 text-rose-700")
                  }
                >
                  {item}
                </span>
              ))}
            </div>
          ) : (
            <p
              className={
                "mt-1 text-[12px] leading-6 " +
                (is_success ? "text-emerald-700/85" : "text-rose-700/85")
              }
            >
              {message}
            </p>
          )}
        </div>
      </div>
    </div>
  );
}

export function SkillsDirectory() {
  const [skills, set_skills] = useState<SkillInfo[]>([]);
  const [search_query, set_search_query] = useState("");
  const [discovery_mode, set_discovery_mode] = useState<DiscoveryMode>("catalog");
  const [source_filter, set_source_filter] = useState<SourceFilter>("all");
  const [active_category, set_active_category] = useState<string>("all");
  const [selected_skill, set_selected_skill] = useState<string | null>(null);
  const [external_query, set_external_query] = useState("");
  const [external_results, set_external_results] = useState<ExternalSkillSearchItem[]>([]);
  const [preview_external_item, set_preview_external_item] = useState<ExternalSkillSearchItem | null>(null);
  const [external_loading, set_external_loading] = useState(false);
  const [git_prompt_open, set_git_prompt_open] = useState(false);
  const [loading, set_loading] = useState(true);
  const [busy_skill_name, set_busy_skill_name] = useState<string | null>(null);
  const [status_message, set_status_message] = useState<string | null>(null);
  const [error_message, set_error_message] = useState<string | null>(null);
  const file_input_ref = useRef<HTMLInputElement | null>(null);

  const load_skills = useCallback(async (query: string, source: SourceFilter) => {
    const next_skills = await getAvailableSkillsApi({
      q: query || undefined,
      source_type: source === "all" ? undefined : source,
    });
    set_skills(next_skills);
  }, []);

  const load_data = useCallback(async () => {
    try {
      set_loading(true);
      await load_skills(search_query, source_filter);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "加载 Skill Marketplace 失败");
    } finally {
      set_loading(false);
    }
  }, [load_skills, search_query, source_filter]);

  useEffect(() => {
    void load_data();
  }, [load_data]);

  useEffect(() => {
    if (discovery_mode !== "catalog") return;
    void load_skills(search_query, source_filter).catch((err) => {
      set_error_message(err instanceof Error ? err.message : "刷新技能列表失败");
    });
  }, [discovery_mode, load_skills, search_query, source_filter]);

  const categories = useMemo(() => {
    const map = new Map<string, string>();
    skills.forEach((skill) => {
      map.set(skill.category_key, skill.category_name);
    });
    return [{ key: "all", label: "全部" }].concat(
      Array.from(map.entries()).map(([key, label]) => ({ key, label })),
    );
  }, [skills]);

  const visible_skills = useMemo(() => {
    if (active_category === "all") return skills;
    return skills.filter((skill) => skill.category_key === active_category);
  }, [active_category, skills]);

  const grouped_skills = useMemo(() => {
    const map = new Map<string, SkillInfo[]>();
    visible_skills.forEach((skill) => {
      const list = map.get(skill.category_name) ?? [];
      list.push(skill);
      map.set(skill.category_name, list);
    });
    return Array.from(map.entries());
  }, [visible_skills]);

  const imported_skill_names = useMemo(
    () => new Set(skills.map((skill) => skill.name)),
    [skills],
  );

  const clear_messages = () => {
    set_status_message(null);
    set_error_message(null);
  };

  const refresh_marketplace = useCallback(async () => {
    await load_skills(search_query, source_filter);
  }, [load_skills, search_query, source_filter]);

  const handle_update_single = useCallback(async (skill_name: string) => {
    clear_messages();
    try {
      set_busy_skill_name(skill_name);
      await updateSingleSkillApi(skill_name);
      set_status_message(`已更新 ${skill_name}`);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "更新 Skill 失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace]);

  const handle_toggle_global_enabled = useCallback(async (skill: SkillInfo) => {
    clear_messages();
    try {
      set_busy_skill_name(skill.name);
      await setSkillGlobalEnabledApi(skill.name, !skill.global_enabled);
      set_status_message(
        `${skill.title || skill.name} 已${skill.global_enabled ? "全局停用" : "全局启用"}`,
      );
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "切换全局启用状态失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace]);

  const handle_delete_from_pool = useCallback(async (skill: SkillInfo) => {
    clear_messages();
    try {
      set_busy_skill_name(skill.name);
      await deleteSkillFromPoolApi(skill.name);
      set_status_message(`${skill.title || skill.name} 已从资源池删除`);
      if (selected_skill === skill.name) {
        set_selected_skill(null);
      }
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "删除资源池 skill 失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace, selected_skill]);

  const handle_update_installed = useCallback(async () => {
    clear_messages();
    try {
      const result = await updateImportedSkillsApi();
      set_status_message(
        `更新完成：更新 ${result.updated_skills.length} 个，跳过 ${result.skipped_skills.length} 个`,
      );
      if (result.failures.length) {
        set_error_message(result.failures.map((item) => `${item.skill_name}: ${item.error}`).join("；"));
      }
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "更新已安装 Skill 失败");
    }
  }, [refresh_marketplace]);

  const handle_local_import = useCallback(async (file: File) => {
    clear_messages();
    try {
      await importLocalSkillApi(file);
      set_status_message(`已导入本地 Skill：${file.name}`);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "本地导入失败");
    }
  }, [refresh_marketplace]);

  const handle_git_import = useCallback(async (url: string) => {
    clear_messages();
    if (!url.trim()) return;
    try {
      await importGitSkillApi(url.trim());
      set_status_message(`已通过 Git 导入 Skill`);
      set_git_prompt_open(false);
      await refresh_marketplace();
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "Git 导入失败");
    }
  }, [refresh_marketplace]);

  const handle_external_search = useCallback(async () => {
    clear_messages();
    if (!external_query.trim()) {
      set_external_results([]);
      return;
    }
    try {
      set_external_loading(true);
      const results = await searchExternalSkillsApi(external_query.trim());
      set_external_results(results);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "外部技能搜索失败");
    } finally {
      set_external_loading(false);
    }
  }, [external_query]);

  const handle_import_external = useCallback(async (
    item: ExternalSkillSearchItem,
  ) => {
    clear_messages();
    try {
      set_busy_skill_name(item.skill_slug);
      await importSkillsShSkillApi(item.package_spec, item.skill_slug);
      set_status_message(`已导入到全局技能库：${item.skill_slug}`);
      await refresh_marketplace();
      set_preview_external_item(null);
    } catch (err) {
      set_error_message(err instanceof Error ? err.message : "从 skills.sh 导入失败");
    } finally {
      set_busy_skill_name(null);
    }
  }, [refresh_marketplace]);

  const formatInstalls = (installs: number) => {
    if (installs >= 1000) {
      return `${(installs / 1000).toFixed(installs >= 100000 ? 0 : 1)}K`;
    }
    return `${installs}`;
  };

  const header_trailing = (
    <>
      <select
        className="workspace-chip rounded-full px-3 py-2 text-[12px] font-semibold text-slate-700"
        onChange={(e) => set_source_filter(e.target.value as SourceFilter)}
        value={source_filter}
      >
        <option value="all">全部来源</option>
        <option value="builtin">内置</option>
        <option value="external">外部</option>
        <option value="system">系统</option>
      </select>
      <WorkspacePillButton onClick={() => file_input_ref.current?.click()} size="sm">
        <FolderUp className="h-3.5 w-3.5" />
        导入本地
      </WorkspacePillButton>
      <WorkspacePillButton onClick={() => set_git_prompt_open(true)} size="sm">
        <Download className="h-3.5 w-3.5" />
        Git 安装
      </WorkspacePillButton>
      <WorkspacePillButton onClick={() => void handle_update_installed()} size="sm">
        <RefreshCw className="h-3.5 w-3.5" />
        更新技能库
      </WorkspacePillButton>
    </>
  );

  return (
    <div className="flex min-h-0 min-w-0 flex-1 flex-col">
      <WorkspaceSurfaceHeader
        active_tab={active_category}
        badge="SKILL MARKETPLACE"
        class_name="sticky top-0"
        leading={<Puzzle className="h-4 w-4 text-slate-800/72" />}
        on_change_tab={set_active_category}
        subtitle="浏览和维护全局技能资源池"
        tabs={categories.map((category) => ({ key: category.key, label: category.label }))}
        title="Skills"
        trailing={header_trailing}
      />

      <input
        accept=".zip,application/zip"
        className="hidden"
        onChange={(e) => {
          const file = e.target.files?.[0];
          if (file) {
            void handle_local_import(file);
          }
          e.currentTarget.value = "";
        }}
        ref={file_input_ref}
        type="file"
      />

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-5 py-5 xl:px-6">
        <div className="workspace-card mb-6 rounded-[28px] px-5 py-5">
          <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <div className="flex items-center gap-2">
                <Sparkles className="h-4 w-4 text-sky-500" />
                <p className="text-[18px] font-black tracking-[-0.03em] text-slate-950/90">
                  发现技能
                </p>
              </div>
              <p className="mt-1 text-sm text-slate-700/60">
                用一个搜索入口切换查看库内技能或搜索 skills.sh 外部目录。
              </p>
            </div>
            <div className="flex w-full flex-col gap-2 lg:w-[720px]">
              <div className="flex flex-wrap gap-2">
                <button
                  className={
                    "rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all " +
                    (discovery_mode === "catalog"
                      ? "workspace-chip text-slate-950"
                      : "text-slate-600 hover:bg-white/50")
                  }
                  onClick={() => set_discovery_mode("catalog")}
                  type="button"
                >
                  库内技能
                </button>
                <button
                  className={
                    "rounded-full px-3 py-1.5 text-[12px] font-semibold transition-all " +
                    (discovery_mode === "external"
                      ? "workspace-chip text-slate-950"
                      : "text-slate-600 hover:bg-white/50")
                  }
                  onClick={() => set_discovery_mode("external")}
                  type="button"
                >
                  skills.sh
                </button>
              </div>
              <div className="flex w-full flex-col gap-2 sm:flex-row">
                <div className="flex-1">
                  <WorkspaceSearchInput
                    input_class_name="w-full"
                    on_change={(value) => {
                      set_search_query(value);
                      if (discovery_mode === "external") {
                        set_external_query(value);
                      }
                    }}
                    placeholder={
                      discovery_mode === "catalog"
                        ? "搜索当前技能库里的 skill、标签或场景..."
                        : "搜索 skills.sh 外部技能，例如 react、playwright、seo"
                    }
                    value={discovery_mode === "catalog" ? search_query : external_query}
                  />
                </div>
                {discovery_mode === "external" ? (
                  <WorkspacePillButton onClick={() => void handle_external_search()} size="md">
                    <Globe2 className="h-4 w-4" />
                    搜索外部技能
                  </WorkspacePillButton>
                ) : null}
              </div>
            </div>
          </div>

          {discovery_mode === "external" && !external_loading && external_query && !external_results.length ? (
            <div className="mt-4 rounded-[20px] border border-dashed border-white/40 bg-white/40 px-4 py-4 text-sm text-slate-500">
              暂无匹配结果，试试更具体的关键词，比如 `react performance`、`playwright e2e`、`seo content`。
            </div>
          ) : null}

          {discovery_mode === "external" && external_loading ? (
            <div className="mt-4 flex items-center gap-2 text-sm text-slate-500">
              <Loader2 className="h-4 w-4 animate-spin" />
              正在搜索 skills.sh...
            </div>
          ) : discovery_mode === "external" && external_results.length ? (
            <>
              <div className="mt-4 flex items-center justify-between gap-3">
                <div className="text-sm text-slate-600">
                  找到 <span className="font-bold text-slate-900">{external_results.length}</span> 个结果
                </div>
                <div className="text-xs text-slate-500">
                  优先展示安装量更高、可直接复用的 skill
                </div>
              </div>
              <div className="mt-5 grid grid-cols-1 gap-3 xl:grid-cols-2">
                {external_results.map((item) => {
                  const already_imported = imported_skill_names.has(item.skill_slug);
                  return (
                    <div
                      key={`${item.package_spec}@${item.skill_slug}`}
                      className="rounded-[22px] border border-white/28 bg-white/60 px-4 py-4 transition-all hover:bg-white/72"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div className="min-w-0">
                          <div className="flex flex-wrap items-center gap-2">
                            <p className="truncate text-[15px] font-bold text-slate-950/90">
                              {item.title || item.skill_slug}
                            </p>
                            <span className="inline-flex rounded-full bg-sky-50 px-2 py-0.5 text-[10px] font-semibold text-sky-600">
                              {formatInstalls(item.installs)} installs
                            </span>
                            {already_imported ? (
                              <span className="inline-flex rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-600">
                                已导入
                              </span>
                            ) : null}
                          </div>
                          <p className="mt-1 truncate text-[12px] text-slate-600">
                            {item.package_spec}
                          </p>
                        </div>
                        <div className="flex items-center gap-2">
                          <WorkspacePillButton
                            onClick={() => set_preview_external_item(item)}
                            size="sm"
                          >
                            预览
                          </WorkspacePillButton>
                          <WorkspacePillButton
                            disabled={busy_skill_name === item.skill_slug}
                            onClick={() => void handle_import_external(item)}
                            size="sm"
                            variant="strong"
                          >
                            {busy_skill_name === item.skill_slug ? (
                              <Loader2 className="h-3.5 w-3.5 animate-spin" />
                            ) : (
                              <Download className="h-3.5 w-3.5" />
                            )}
                            导入到技能库
                          </WorkspacePillButton>
                        </div>
                      </div>
                      <p className="mt-3 line-clamp-3 text-[13px] leading-6 text-slate-700/72">
                        {item.readme_markdown || item.description}
                      </p>
                      <div className="mt-4 flex items-center gap-2 text-[11px] text-slate-500">
                        <Import className="h-3.5 w-3.5" />
                        导入后将由 Nexus 自己管理安装与更新
                      </div>
                    </div>
                  );
                })}
              </div>
            </>
          ) : discovery_mode === "catalog" ? (
            <div className="mt-4 flex items-center gap-2 text-xs text-slate-500">
              <Search className="h-3.5 w-3.5" />
              当前搜索只过滤库内技能列表，不会触发外部搜索。
            </div>
          ) : null}
        </div>

        {status_message ? (
          <FeedbackBanner
            message={status_message}
            title="操作完成"
            tone="success"
          />
        ) : null}
        {error_message ? (
          <FeedbackBanner
            message={error_message}
            title="部分操作失败"
            tone="error"
          />
        ) : null}

        {loading ? (
          <div className="flex min-h-[320px] items-center justify-center text-sm text-slate-500/60">
            <Loader2 className="h-5 w-5 animate-spin" />
          </div>
        ) : !grouped_skills.length ? (
          <div className="workspace-card flex min-h-[320px] items-center justify-center rounded-[28px] px-8 text-center">
            <div>
              <p className="text-[22px] font-bold tracking-[-0.04em] text-slate-950/90">
                没有符合条件的 Skill
              </p>
              <p className="mt-3 text-sm leading-7 text-slate-700/60">
                试试切换分类、来源或搜索条件。
              </p>
            </div>
          </div>
        ) : (
          <div className="space-y-8">
            {grouped_skills.map(([category_name, items]) => (
              <section key={category_name}>
                <div className="mb-4 flex items-center gap-3">
                  <h2 className="text-[18px] font-black tracking-[-0.03em] text-slate-950/90">
                    {category_name}
                  </h2>
                  <span className="rounded-full bg-white/70 px-2 py-0.5 text-[11px] font-semibold text-slate-500">
                    {items.length}
                  </span>
                </div>
                <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-3">
                  {items.map((skill) => {
                    const is_busy = busy_skill_name === skill.name;
                    return (
                      <div key={skill.name} className={is_busy ? "opacity-70" : ""}>
                        <SkillsCard
                          busy={is_busy}
                          on_delete={() => void handle_delete_from_pool(skill)}
                          on_select={() => set_selected_skill(skill.name)}
                          on_toggle_global_enabled={() => void handle_toggle_global_enabled(skill)}
                          on_update={() => void handle_update_single(skill.name)}
                          skill={skill}
                        />
                      </div>
                    );
                  })}
                </div>
              </section>
            ))}
          </div>
        )}
      </div>

      {selected_skill ? (
        <SkillDetailDialog
          is_open={!!selected_skill}
          on_close={() => set_selected_skill(null)}
          on_refresh={refresh_marketplace}
          skill_name={selected_skill}
        />
      ) : null}

      <PromptDialog
        default_value=""
        is_open={git_prompt_open}
        message="输入包含 SKILL.md 的 Git 仓库地址。第一版默认拉取默认分支。"
        on_cancel={() => set_git_prompt_open(false)}
        on_confirm={(value) => {
          void handle_git_import(value);
        }}
        placeholder="https://github.com/owner/repo.git"
        title="通过 Git 安装 Skill"
      />
      <ExternalSkillPreviewDialog
        already_imported={preview_external_item ? imported_skill_names.has(preview_external_item.skill_slug) : false}
        busy={!!preview_external_item && busy_skill_name === preview_external_item.skill_slug}
        is_open={!!preview_external_item}
        item={preview_external_item}
        on_close={() => set_preview_external_item(null)}
        on_import_only={() => {
          if (preview_external_item) {
            void handle_import_external(preview_external_item);
          }
        }}
      />
    </div>
  );
}
