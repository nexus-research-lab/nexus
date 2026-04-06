"use client";

import { Check, Lock, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBadge,
  WorkspaceCatalogCard,
  WorkspaceIconFrame,
  WorkspaceCatalogTag,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { SkillInfo } from "@/types/skill";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  on_select: () => void;
  on_update?: () => void;
  on_delete?: () => void;
}

/** Skill 卡片 — 清晰的三段式布局 */
export function SkillsCard({
  skill,
  busy = false,
  on_select,
  on_update,
  on_delete,
}: SkillsCardProps) {
  const {
    title,
    description,
    locked,
    tags,
    source_type,
    has_update,
    deletable,
  } = skill;

  const source_label =
    source_type === "system" ? "系统" : source_type === "builtin" ? "内置" : "外部";

  return (
    <WorkspaceCatalogCard
      class_name="group cursor-pointer rounded-[22px] px-5 py-4"
      onClick={on_select}
    >
      {/* 头部：图标 + 名称 + 来源标签 */}
      <div className="flex items-center gap-3">
        <WorkspaceIconFrame
          class_name={cn("h-10 w-10 shrink-0", source_type === "external" && "text-sky-600")}
          size="md"
          tone={locked ? "warning" : source_type === "external" ? "primary" : "default"}
        >
          {locked ? <Lock className="h-[18px] w-[18px]" /> : <Puzzle className="h-[18px] w-[18px]" />}
        </WorkspaceIconFrame>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <p className="truncate text-[15px] font-bold tracking-[-0.02em] text-slate-950/98">
              {title}
            </p>
            <WorkspaceCatalogBadge class_name="shrink-0" tone="neutral">
              {source_label}
            </WorkspaceCatalogBadge>
            {has_update && (
              <WorkspaceCatalogBadge class_name="shrink-0" tone="info">
                可更新
              </WorkspaceCatalogBadge>
            )}
          </div>
        </div>
      </div>

      {/* 描述 */}
      <p className="mt-2.5 line-clamp-2 flex-1 text-[13px] leading-[1.55] text-slate-700/78">
        {description || "暂无描述"}
      </p>

      {/* 底部：标签 + 状态 */}
      <div className="mt-3 flex items-end justify-between gap-3">
        {/* 标签 */}
        <div className="flex min-w-0 flex-wrap gap-1">
          {tags.slice(0, 2).map((tag) => (
            <WorkspaceCatalogTag key={tag}>
              {tag}
            </WorkspaceCatalogTag>
          ))}
        </div>

        <div className="flex shrink-0 items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
          {locked ? (
            <WorkspaceCatalogBadge tone="warning">
              <Lock className="h-3 w-3" />
              系统托管
            </WorkspaceCatalogBadge>
          ) : source_type === "external" ? (
            <WorkspaceCatalogBadge tone="success">
              <Check className="h-3 w-3" />
              已导入
            </WorkspaceCatalogBadge>
          ) : (
            <WorkspacePillButton density="compact" size="sm" variant="outlined">
              <Puzzle className="h-3 w-3" />
              可安装到 Agent
            </WorkspacePillButton>
          )}
          {has_update ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_update}
              title="更新"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </WorkspaceCatalogAction>
          ) : null}
          {deletable ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_delete}
              title="从技能库删除"
              tone="danger"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </WorkspaceCatalogAction>
          ) : null}
        </div>
      </div>
    </WorkspaceCatalogCard>
  );
}
