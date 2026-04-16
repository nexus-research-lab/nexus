"use client";

import { Lock, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  WorkspaceCatalogAction,
  WorkspaceCatalogBody,
  WorkspaceCatalogCard,
  WorkspaceCatalogDescription,
  WorkspaceCatalogFooter,
  WorkspaceCatalogHeader,
  WorkspaceIconFrame,
  WorkspaceCatalogTag,
  WorkspaceCatalogTitle,
} from "@/shared/ui/workspace/catalog/workspace-catalog-card";
import { SkillInfo } from "@/types/capability/skill";
import { SkillStatePill } from "./skill-state-pill";

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
    source_type === "system" ? "系统内置" : source_type === "builtin" ? "内置推荐" : "外部导入";
  const visible_tags = tags.slice(0, 2);
  const state_label = locked ? "系统托管" : source_type === "external" ? "已导入" : "可安装";
  const state_tone = locked ? "warning" : source_type === "external" ? "success" : "neutral";

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
      <WorkspaceCatalogHeader class_name="items-center gap-3.5">
        <WorkspaceIconFrame
          class_name={cn("shrink-0", source_type === "external" && "text-sky-600")}
          size="sm"
          tone={locked ? "warning" : source_type === "external" ? "primary" : "default"}
        >
          {locked ? <Lock className="h-4 w-4" /> : <Puzzle className="h-4 w-4" />}
        </WorkspaceIconFrame>
        <div className="min-w-0 flex-1">
          <div className="min-w-0">
            <WorkspaceCatalogTitle class_name="min-w-0" size="sm" truncate>
              {title}
            </WorkspaceCatalogTitle>
            <div className="mt-1 flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-(--text-soft)">
              <span>{source_label}</span>
              {has_update ? (
                <>
                  <span className="opacity-35">·</span>
                  <span>有更新</span>
                </>
              ) : null}
            </div>
          </div>
        </div>
      </WorkspaceCatalogHeader>

      <WorkspaceCatalogBody grow>
        <WorkspaceCatalogDescription min_height>
          {description || "暂无描述"}
        </WorkspaceCatalogDescription>
      </WorkspaceCatalogBody>

      <WorkspaceCatalogFooter justify={visible_tags.length ? "between" : "end"}>
        {visible_tags.length ? (
          <div className="flex min-w-0 flex-wrap items-center gap-1.5">
            {visible_tags.map((tag) => (
              <WorkspaceCatalogTag key={tag} class_name="max-w-full px-2.5 text-[10px] text-(--text-soft)">
                {tag}
              </WorkspaceCatalogTag>
            ))}
          </div>
        ) : null}

        <div className="flex shrink-0 items-center gap-1.5" onClick={(e) => e.stopPropagation()}>
          <SkillStatePill tone={state_tone}>
            {state_label}
          </SkillStatePill>
          {has_update ? (
            <WorkspaceCatalogAction
              disabled={busy}
              onClick={on_update}
              size="sm"
              title="更新"
            >
              <RefreshCw className="h-3 w-3" />
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
              <Trash2 className="h-3 w-3" />
            </WorkspaceCatalogAction>
          ) : null}
        </div>
      </WorkspaceCatalogFooter>
    </WorkspaceCatalogCard>
  );
}
