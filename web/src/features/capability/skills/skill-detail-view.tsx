"use client";

import { useCallback, useEffect, useState } from "react";
import {
  ArrowLeft,
  ChevronRight,
  ExternalLink,
  Loader2,
  Lock,
  Puzzle,
  RefreshCw,
  Trash2,
} from "lucide-react";

import { delete_skill_api, get_skill_detail_api, update_single_skill_api } from "@/lib/api/skill-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { UiPanel } from "@/shared/ui/panel";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { SkillDetail } from "@/types/capability/skill";

import { SkillMarkdown } from "./skill-markdown";

interface SkillDetailViewProps {
  skill_name: string;
  on_back: () => void;
  on_deleted: () => Promise<void> | void;
  on_refreshed: () => Promise<void> | void;
}

function get_skill_source_label(skill: SkillDetail): string {
  if (skill.source_type === "system") return "系统内置";
  if (skill.source_type === "builtin") return "内置推荐";
  if (skill.source_type === "external") return "用户导入";
  return "工作区技能";
}

/** Skill 详情页 —— 与连接器详情同样使用路由承载主体内容。 */
export function SkillDetailView({
  skill_name,
  on_back,
  on_deleted,
  on_refreshed,
}: SkillDetailViewProps) {
  const [skill, set_skill] = useState<SkillDetail | null>(null);
  const [loading, set_loading] = useState(true);
  const [acting, set_acting] = useState(false);
  const [error, set_error] = useState<string | null>(null);
  const source_url = skill?.source_ref && /^https?:\/\//.test(skill.source_ref) ? skill.source_ref : null;

  const load_detail = useCallback(async () => {
    try {
      set_loading(true);
      set_error(null);
      set_skill(await get_skill_detail_api(skill_name));
    } catch (err) {
      set_error(err instanceof Error ? err.message : "加载 skill 详情失败");
      set_skill(null);
    } finally {
      set_loading(false);
    }
  }, [skill_name]);

  useEffect(() => {
    void load_detail();
  }, [load_detail]);

  const handle_update = useCallback(async () => {
    if (!skill) return;
    try {
      set_acting(true);
      set_error(null);
      await update_single_skill_api(skill.name);
      await Promise.resolve(on_refreshed());
      await load_detail();
    } catch (err) {
      set_error(err instanceof Error ? err.message : "更新 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [load_detail, on_refreshed, skill]);

  const handle_delete = useCallback(async () => {
    if (!skill || !skill.deletable) return;
    try {
      set_acting(true);
      set_error(null);
      await delete_skill_api(skill.name);
      await Promise.resolve(on_deleted());
    } catch (err) {
      set_error(err instanceof Error ? err.message : "删除 skill 失败");
    } finally {
      set_acting(false);
    }
  }, [on_deleted, skill]);

  return (
    <div className="mx-auto w-full max-w-[980px] px-5 py-6 xl:px-6">
      <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
        <button
          className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
          onClick={on_back}
          type="button"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          技能
        </button>
        {skill ? (
          <>
            <ChevronRight className="h-3.5 w-3.5 text-(--icon-muted)" />
            <span className="truncate font-medium text-(--text-strong)">{skill.title || skill.name}</span>
          </>
        ) : null}
      </div>

      {loading ? (
        <UiStateBlock
          class_name="min-h-[420px]"
          icon={<Loader2 className="h-6 w-6 animate-spin" />}
          size="md"
          title="加载技能详情中..."
          variant="plain"
        />
      ) : !skill ? (
        <UiStateBlock
          actions={(
            <UiButton onClick={on_back} size="sm" type="button">
              返回技能
            </UiButton>
          )}
          class_name="min-h-[420px]"
          description={error}
          size="md"
          title="技能不存在"
          tone={error ? "danger" : "default"}
          variant="plain"
        />
      ) : (
        <div className="pt-9">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="flex min-w-0 items-start gap-4">
              <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-(--surface-panel-background) text-(--icon-default) shadow-[0_1px_3px_rgba(15,23,42,0.08)]">
                {skill.locked ? <Lock className="h-6 w-6" /> : <Puzzle className="h-6 w-6" />}
              </div>
              <div className="min-w-0">
                <h1 className="text-[24px] font-semibold tracking-[-0.035em] text-(--text-strong)">
                  {skill.title || skill.name}{" "}
                  <span className="ml-2 font-normal text-(--text-muted)">Skill</span>
                </h1>
                <p className="mt-2 max-w-[720px] text-[15px] leading-6 text-(--text-muted)">
                  {skill.description || "暂无描述"}
                </p>
              </div>
            </div>

            <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
              {skill.source_type === "external" && skill.has_update ? (
                <UiButton
                  disabled={acting}
                  onClick={() => void handle_update()}
                  size="sm"
                  tone="primary"
                  type="button"
                  variant="solid"
                >
                  {acting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RefreshCw className="h-3.5 w-3.5" />}
                  更新技能
                </UiButton>
              ) : null}
              {skill.deletable ? (
                <UiButton
                  disabled={acting}
                  onClick={() => void handle_delete()}
                  size="sm"
                  tone="danger"
                  type="button"
                  variant="surface"
                >
                  {acting ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
                  删除
                </UiButton>
              ) : null}
            </div>
          </div>

          <div className="mt-8 max-w-[820px] space-y-6">
            <div className="flex flex-wrap gap-2">
              <UiBadge>{skill.category_name}</UiBadge>
              <UiBadge>{get_skill_source_label(skill)}</UiBadge>
              <UiBadge>版本 {skill.version || "unknown"}</UiBadge>
              {skill.locked ? <UiBadge tone="warning">系统锁定</UiBadge> : null}
              {skill.tags.map((tag) => (
                <UiBadge key={tag}>{tag}</UiBadge>
              ))}
            </div>

            {error ? (
              <UiStateBlock description={error} size="sm" title="操作失败" tone="danger" />
            ) : null}

            <section>
              <h2 className="mb-3 text-[16px] font-semibold tracking-[-0.025em] text-(--text-strong)">
                技能说明
              </h2>
              <UiPanel padding="md" radius="md" variant="inset">
                <SkillMarkdown
                  description={skill.description}
                  markdown={skill.readme_markdown}
                  title={skill.title || skill.name}
                />
              </UiPanel>
            </section>

            {source_url ? (
              <a
                className="inline-flex items-center gap-2 text-[13px] font-semibold text-(--primary) underline decoration-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] underline-offset-4"
                href={source_url}
                rel="noopener noreferrer"
                target="_blank"
              >
                <ExternalLink className="h-3.5 w-3.5" />
                查看来源
              </a>
            ) : null}
          </div>
        </div>
      )}
    </div>
  );
}
