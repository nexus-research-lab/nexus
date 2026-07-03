import { Loader2, Puzzle } from "lucide-react";

import type { SkillInfo } from "@/types/capability/skill";

import { SkillsCard } from "./skills-card";
import type { SkillMarketplaceController } from "./skills-view-model";

interface SkillsCatalogGridProps {
  ctrl: SkillMarketplaceController;
  onOpenSkill: (skillName: string) => void;
}

export function SkillsCatalogGrid({ ctrl, onOpenSkill: onOpenSkill }: SkillsCatalogGridProps) {
  if (ctrl.loading) {
    return (
      <div className="flex min-h-80 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-(--text-muted)" />
      </div>
    );
  }

  if (!ctrl.groupedSkills.length) {
    return (
      <div className="flex min-h-80 flex-col items-center justify-center gap-3 text-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-full border border-(--divider-subtle-color) bg-transparent">
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
    <div className="space-y-9">
      {ctrl.groupedSkills.map(([categoryName, items]: [string, SkillInfo[]]) => (
        <section key={categoryName}>
          <div className="mb-3 flex items-end justify-between border-b border-(--divider-subtle-color) pb-2">
            <h2 className="text-[18px] font-medium tracking-[-0.025em] text-(--text-strong)">
              {categoryName}
            </h2>
            <span className="text-[12px] font-medium text-(--text-soft)">
              {items.length} 个
            </span>
          </div>
          <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
            {items.map((skill: SkillInfo) => (
              <SkillsCard
                key={skill.name}
                busy={ctrl.busySkillName === skill.name}
                className="transition-opacity"
                onDelete={() => void ctrl.handleDeleteSkill(skill)}
                onSelect={() => onOpenSkill(skill.name)}
                onUpdate={() => void ctrl.handleUpdateSingle(skill.name)}
                skill={skill}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
