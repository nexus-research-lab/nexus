"use client";

import { Lock, Puzzle, RefreshCw, Trash2 } from "lucide-react";

import { cn } from "@/lib/utils";
import { UiBadge } from "@/shared/ui/badge";
import { UiListActionButton } from "@/shared/ui/list-action";
import { UiListRow } from "@/shared/ui/list-row";
import type { SkillInfo } from "@/types/capability/skill";
import { SkillStatePill } from "./skill-state-pill";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  class_name?: string;
  on_select: () => void;
  on_update?: () => void;
  on_delete?: () => void;
}

/** Skill 行 —— 与连接器目录保持一致的轻量列表结构。 */
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
    <UiListRow
      class_name={cn(
        "min-h-[72px] rounded-[14px] px-2 py-1.5",
        busy && "opacity-60",
        class_name,
      )}
      leading={(
        <span
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-(--surface-panel-background) shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
            locked && "text-amber-700",
            source_type === "external" && "text-sky-600",
          )}
        >
          {locked ? <Lock className="h-4 w-4" /> : <Puzzle className="h-4 w-4" />}
        </span>
      )}
      on_click={on_select}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <SkillStatePill tone={state_tone}>{state_label}</SkillStatePill>
          {has_update ? (
            <UiListActionButton
              disabled={busy}
              onClick={on_update}
              size="sm"
              stop_propagation
              title="更新"
            >
              <RefreshCw className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
          {deletable ? (
            <UiListActionButton
              disabled={busy}
              onClick={on_delete}
              size="sm"
              stop_propagation
              title="从技能库删除"
              tone="danger"
            >
              <Trash2 className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
        </div>
      )}
    >
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-2">
          <span className="truncate text-[15px] font-semibold tracking-[-0.02em] text-(--text-strong)">
            {title}
          </span>
          {has_update ? <UiBadge size="xs" tone="warning">有更新</UiBadge> : null}
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {description || "暂无描述"}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-(--text-soft)">
          <span className="shrink-0">{source_label}</span>
          {visible_tags.map((tag) => (
            <span key={tag} className="truncate">
              · {tag}
            </span>
          ))}
        </div>
      </div>
    </UiListRow>
  );
}
