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
  className?: string;
  onSelect: () => void;
  onUpdate?: () => void;
  onDelete?: () => void;
}

/** Skill 行 —— 与连接器目录保持一致的轻量列表结构。 */
export function SkillsCard({
  skill,
  busy = false,
  className: className,
  onSelect: onSelect,
  onUpdate: onUpdate,
  onDelete: onDelete,
}: SkillsCardProps) {
  const {
    title,
    description,
    locked,
    tags,
    source_type: sourceType,
    has_update: hasUpdate,
    deletable,
  } = skill;

  const sourceLabel =
    sourceType === "system" ? "系统内置" : sourceType === "builtin" ? "内置推荐" : "外部导入";
  const visibleTags = tags.slice(0, 2);
  const stateLabel = locked ? "系统托管" : sourceType === "external" ? "已导入" : "可安装";
  const stateTone = locked ? "warning" : sourceType === "external" ? "success" : "neutral";

  return (
    <UiListRow
      className={cn(
        "min-h-[72px] rounded-[14px] px-2 py-1.5",
        busy && "opacity-60",
        className,
      )}
      leading={(
        <span
          className={cn(
            "flex h-11 w-11 shrink-0 items-center justify-center rounded-[12px] border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_70%,transparent)] bg-(--surface-panel-background) shadow-[0_1px_2px_rgba(15,23,42,0.04)]",
            locked && "text-(--warning)",
            sourceType === "external" && "text-(--status-info-soft-text)",
          )}
        >
          {locked ? <Lock className="h-4 w-4" /> : <Puzzle className="h-4 w-4" />}
        </span>
      )}
      onClick={onSelect}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <SkillStatePill tone={stateTone}>{stateLabel}</SkillStatePill>
          {hasUpdate ? (
            <UiListActionButton
              disabled={busy}
              onClick={onUpdate}
              size="sm"
              stopPropagation
              title="更新"
            >
              <RefreshCw className="h-3 w-3" />
            </UiListActionButton>
          ) : null}
          {deletable ? (
            <UiListActionButton
              disabled={busy}
              onClick={onDelete}
              size="sm"
              stopPropagation
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
          {hasUpdate ? <UiBadge size="xs" tone="warning">有更新</UiBadge> : null}
        </div>
        <div className="mt-0.5 truncate text-[13px] leading-5 text-(--text-muted)">
          {description || "暂无描述"}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] leading-4 text-(--text-soft)">
          <span className="shrink-0">{sourceLabel}</span>
          {visibleTags.map((tag) => (
            <span key={tag} className="truncate">
              · {tag}
            </span>
          ))}
        </div>
      </div>
    </UiListRow>
  );
}
