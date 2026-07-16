"use client";

import { Lock, Puzzle, Trash2 } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { UiBadge } from "@/shared/ui/display/badge";
import { UiListActionButton } from "@/shared/ui/list/list-action";
import { UiListRow } from "@/shared/ui/list/list-row";
import type { SkillInfo } from "@/types/capability/skill";

import {
  buildSkillCardModel,
  type SkillCatalogIcon,
} from "./skills-catalog-model";

interface SkillsCardProps {
  skill: SkillInfo;
  busy?: boolean;
  className?: string;
  onSelect: () => void;
  onDelete?: () => void;
}

const SKILL_CARD_ICON = {
  lock: Lock,
  puzzle: Puzzle,
} satisfies Record<SkillCatalogIcon, typeof Puzzle>;

/** Skill 行 —— 与连接器目录保持一致的轻量列表结构。 */
export function SkillsCard({
  skill,
  busy = false,
  className,
  onSelect,
  onDelete,
}: SkillsCardProps) {
  const model = buildSkillCardModel(skill);
  const Icon = SKILL_CARD_ICON[model.icon];

  return (
    <UiListRow
      className={cn(
        "min-h-[64px] rounded-[8px] px-2 py-1",
        busy && "opacity-60",
        className,
      )}
      leading={(
        <span
          className={cn(
            "flex h-9 w-9 shrink-0 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-panel-background)",
            model.iconClassName,
          )}
        >
          <Icon className="h-4 w-4" />
        </span>
      )}
      onClick={onSelect}
      right={(
        <div className="flex shrink-0 items-center gap-1.5">
          <UiBadge tone={model.stateTone}>{model.stateLabel}</UiBadge>
          {model.showDelete ? (
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
          <span className="truncate text-[14px] font-medium text-(--text-strong)">
            {model.title}
          </span>
          {model.showUpdate ? <UiBadge size="xs" tone="warning">有更新</UiBadge> : null}
        </div>
        <div className="mt-0.5 truncate text-[12px] leading-[1.125rem] text-(--text-muted)">
          {model.description}
        </div>
        <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-[10px] leading-4 text-(--text-soft)">
          <span className="shrink-0">{model.sourceLabel}</span>
          {model.visibleTags.map((tag) => (
            <span key={tag} className="truncate">
              · {tag}
            </span>
          ))}
        </div>
      </div>
    </UiListRow>
  );
}
