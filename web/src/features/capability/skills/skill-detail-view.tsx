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

import { cn } from "@/lib/utils";
import { deleteSkillApi, getSkillDetailApi, updateSingleSkillApi } from "@/lib/api/skill-api";
import { UiBadge } from "@/shared/ui/badge";
import { UiButton } from "@/shared/ui/button";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { UiPanel } from "@/shared/ui/panel";
import { UiStateBlock } from "@/shared/ui/state-block";
import type { SkillDetail } from "@/types/capability/skill";

import { SkillMarkdown } from "./skill-markdown";

interface SkillDetailViewProps {
  skillName: string;
  onBack: () => void;
  onDeleted: () => Promise<void> | void;
  onRefreshed: () => Promise<void> | void;
}

function getSkillSourceLabel(skill: SkillDetail): string {
  if (skill.source_type === "system") return "系统内置";
  if (skill.source_type === "builtin") return "内置推荐";
  if (skill.source_type === "external") return "用户导入";
  return "工作区技能";
}

function isHttpSource(value: string): boolean {
  return value.startsWith("http:") || value.startsWith("https:");
}

/** Skill 详情页 —— 与连接器详情同样使用路由承载主体内容。 */
export function SkillDetailView({
  skillName: skillName,
  onBack: onBack,
  onDeleted: onDeleted,
  onRefreshed: onRefreshed,
}: SkillDetailViewProps) {
  const [skill, setSkill] = useState<SkillDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [acting, setActing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const sourceUrl = skill?.source_ref && isHttpSource(skill.source_ref) ? skill.source_ref : null;

  const loadDetail = useCallback(async () => {
    try {
      setLoading(true);
      setError(null);
      setSkill(await getSkillDetailApi(skillName));
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载 skill 详情失败");
      setSkill(null);
    } finally {
      setLoading(false);
    }
  }, [skillName]);

  useEffect(() => {
    void loadDetail();
  }, [loadDetail]);

  const handleUpdate = useCallback(async () => {
    if (!skill) return;
    try {
      setActing(true);
      setError(null);
      await updateSingleSkillApi(skill.name);
      await Promise.resolve(onRefreshed());
      await loadDetail();
    } catch (err) {
      setError(err instanceof Error ? err.message : "更新 skill 失败");
    } finally {
      setActing(false);
    }
  }, [loadDetail, onRefreshed, skill]);

  const handleDelete = useCallback(async () => {
    if (!skill || !skill.deletable) return;
    try {
      setActing(true);
      setError(null);
      await deleteSkillApi(skill.name);
      await Promise.resolve(onDeleted());
    } catch (err) {
      setError(err instanceof Error ? err.message : "删除 skill 失败");
    } finally {
      setActing(false);
    }
  }, [onDeleted, skill]);

  return (
    <div className={WORKSPACE_DETAIL_PAGE_CLASS_NAME}>
      <div className="flex items-center gap-2 text-[14px] text-(--text-muted)">
        <button
          className="inline-flex items-center gap-1 rounded-full px-2 py-1 font-medium transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_28%,transparent)]"
          onClick={onBack}
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
          className="min-h-[420px]"
          icon={<Loader2 className="h-6 w-6 animate-spin" />}
          size="md"
          title="加载技能详情中..."
          variant="plain"
        />
      ) : !skill ? (
        <UiStateBlock
          actions={(
            <UiButton onClick={onBack} size="sm" type="button">
              返回技能
            </UiButton>
          )}
          className="min-h-[420px]"
          description={error}
          size="md"
          title="技能不存在"
          tone={error ? "danger" : "default"}
          variant="plain"
        />
      ) : (
        <div className="pt-9">
          <div className="flex flex-wrap items-start justify-between gap-4">
            <div className="min-w-0 flex-1">
              <div className="flex min-w-0 items-center gap-4">
                <div
                  className={cn(
                    "flex h-16 w-16 shrink-0 items-center justify-center rounded-[18px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-(--surface-panel-background) shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
                    skill.locked ? "text-(--warning)" : skill.source_type === "external" ? "text-(--status-info-soft-text)" : "text-(--icon-default)",
                  )}
                >
                  {skill.locked ? <Lock className="h-9 w-9" /> : <Puzzle className="h-9 w-9" />}
                </div>
                <h1 className="min-w-0 text-[24px] font-semibold tracking-[-0.035em] text-(--text-strong)">
                  <span className="truncate">{skill.title || skill.name}</span>{" "}
                  <span className="font-normal text-(--text-muted)">Skill</span>
                </h1>
              </div>
              <p className="mt-4 text-[15px] leading-6 text-(--text-muted)">
                {skill.description || "暂无描述"}
              </p>
            </div>

            <div className="flex shrink-0 flex-wrap items-center justify-end gap-2">
              {skill.source_type === "external" && skill.has_update ? (
                <UiButton
                  disabled={acting}
                  onClick={() => void handleUpdate()}
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
                  onClick={() => void handleDelete()}
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

          <div className="mt-8 space-y-6">
            <div className="flex flex-wrap gap-2">
              <UiBadge>{skill.category_name}</UiBadge>
              <UiBadge>{getSkillSourceLabel(skill)}</UiBadge>
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

            {sourceUrl ? (
              <a
                className="inline-flex items-center gap-2 text-[13px] font-semibold text-(--primary) underline decoration-[color:color-mix(in_srgb,var(--primary)_28%,transparent)] underline-offset-4"
                href={sourceUrl}
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
