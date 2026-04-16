import { Fragment } from "react";
import { Loader2, Puzzle } from "lucide-react";

import type { SkillInfo } from "@/types/capability/skill";

import type { SkillMarketplaceController } from "@/hooks/capability/use-skill-marketplace";

import { SkillsCard } from "./skills-card";

interface SkillsCatalogGridProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsCatalogGrid({ ctrl }: SkillsCatalogGridProps) {
  if (ctrl.loading) {
    return (
      <div className="flex min-h-80 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-(--text-muted)" />
      </div>
    );
  }

  if (!ctrl.grouped_skills.length) {
    return (
      <div className="flex min-h-80 flex-col items-center justify-center gap-3 text-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-(--surface-inset-background)">
          <Puzzle className="h-6 w-6 text-(--text-muted)" />
        </div>
        <div>
          <p className="text-[16px] font-bold text-(--text-default)">没有符合条件的技能</p>
          <p className="mt-1 text-[13px] text-(--text-soft)">
            试试切换分类、来源或搜索条件
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-7">
      {ctrl.grouped_skills.map(([category_name, items]: [string, SkillInfo[]]) => (
        <Fragment key={category_name}>
          <div className="mb-3 flex items-center gap-2.5">
            <h2 className="text-[15px] font-bold tracking-[-0.02em] text-(--text-strong)">
              {category_name}
            </h2>
            <span className="text-[11px] font-medium text-(--text-soft)">
              {items.length} 个
            </span>
          </div>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
            {items.map((skill: SkillInfo) => (
              <SkillsCard
                key={skill.name}
                busy={ctrl.busy_skill_name === skill.name}
                class_name="transition-opacity"
                on_delete={() => void ctrl.handle_delete_skill(skill)}
                on_select={() => ctrl.set_selected_skill(skill.name)}
                on_update={() => void ctrl.handle_update_single(skill.name)}
                skill={skill}
              />
            ))}
          </div>
        </Fragment>
      ))}
    </div>
  );
}
