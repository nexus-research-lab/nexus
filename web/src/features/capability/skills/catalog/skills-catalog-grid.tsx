import { Loader2, Puzzle } from "lucide-react";

import type { SkillInfo } from "@/types/capability/skill";

import { SkillsCard } from "./skills-card";

interface SkillsCatalogGridProps {
  busySkillNames: ReadonlySet<string>;
  groupedSkills: Array<[string, SkillInfo[]]>;
  loading: boolean;
  onDeleteSkill: (skill: SkillInfo) => void;
  onOpenSkill: (skillName: string) => void;
}

export function SkillsCatalogGrid({
  busySkillNames,
  groupedSkills,
  loading,
  onDeleteSkill,
  onOpenSkill,
}: SkillsCatalogGridProps) {
  if (loading) {
    return (
      <div className="flex min-h-80 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-(--text-muted)" />
      </div>
    );
  }

  if (!groupedSkills.length) {
    return (
      <div className="flex min-h-48 flex-col items-center justify-center gap-2 text-center">
        <div className="flex h-10 w-10 items-center justify-center rounded-[8px] border border-(--divider-subtle-color) bg-transparent">
          <Puzzle className="h-4 w-4 text-(--text-muted)" />
        </div>
        <div>
          <p className="text-[14px] font-medium text-(--text-default)">没有符合条件的技能</p>
          <p className="mt-0.5 text-[12px] text-(--text-soft)">
            试试切换分类、来源或搜索条件
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {groupedSkills.map(([categoryName, items]) => (
        <section key={categoryName}>
          <div className="mb-2 flex items-end justify-between border-b border-(--divider-subtle-color) pb-1.5">
            <h2 className="text-[15px] font-medium text-(--text-strong)">
              {categoryName}
            </h2>
            <span className="text-[11px] font-medium text-(--text-soft)">
              {items.length} 个
            </span>
          </div>
          <div className="grid grid-cols-1 gap-x-8 gap-y-2 md:grid-cols-2">
            {items.map((skill: SkillInfo) => (
              <SkillsCard
                key={skill.name}
                busy={busySkillNames.has(skill.name)}
                className="transition-opacity"
                onDelete={() => onDeleteSkill(skill)}
                onSelect={() => onOpenSkill(skill.name)}
                skill={skill}
              />
            ))}
          </div>
        </section>
      ))}
    </div>
  );
}
