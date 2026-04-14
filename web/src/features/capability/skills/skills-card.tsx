"use client";

import { Check, Lock, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBadge,
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceIconFrame,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/workspace-catalog-card";
import { SkillInfo } from "@/types/skill";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  class_name?: string;
  on_select: () => void;
  on_update?: () => void;
  on_delete?: () => void;
}

/** Skill 卡片 — 清晰的三段式布局 */
export function SkillsCard({
  skill,
  busy = false,
  class_name,
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
  const tag_summary = tags.slice(0, 2).join(" · ");

  return (
    <WorkspaceCatalogCard
      class_name={cn(
        "group h-full",
        busy && "opacity-60",
        class_name,
      )}
      interactive
      onClick={on_select}
      size="catalog"
    >
      <WorkspaceCatalogHeader class_name="items-center">
        <WorkspaceIconFrame
          class_name={cn("h-10 w-10 shrink-0", source_type === "external" && "text-sky-600")}
          size="md"
          tone={locked ? "warning" : source_type === "external" ? "primary" : "default"}
        >
          {locked ? <Lock className="h-[18px] w-[18px]" /> : <Puzzle className="h-[18px] w-[18px]" />}
        </WorkspaceIconFrame>
        <div className="min-w-0 flex-1">
          <div className="min-w-0">
            <WorkspaceCatalogTitle class_name="min-w-0" size="sm" truncate>
              {title}
            </WorkspaceCatalogTitle>
            <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-(--text-soft)">
              <span>{source_label}</span>
              {has_update ? <span>可更新</span> : null}
            </div>
          </div>
        </div>
      </WorkspaceCatalogHeader>

      <WorkspaceCatalogBody grow>
        <WorkspaceCatalogDescription min_height>
          {description || "暂无描述"}
        </WorkspaceCatalogDescription>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter>
        <div className="min-w-0 text-[11px] text-(--text-soft)">
          {tag_summary || "无额外标签"}
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
            <WorkspaceCatalogBadge tone="neutral">
              <Puzzle className="h-3 w-3" />
              可安装到 Agent
            </WorkspaceCatalogBadge>
          )}
          {has_update ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_update}
              size="sm"
              title="更新"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </WorkspaceCatalogAction>
          ) : null}
          {deletable ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_delete}
              size="sm"
              title="从技能库删除"
              tone="danger"
            >
              <Trash2 className="h-3.5 w-3.5" />
            </WorkspaceCatalogAction>
          ) : null}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
