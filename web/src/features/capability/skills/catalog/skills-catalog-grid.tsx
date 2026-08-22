/**
 * INPUT: 已分组 Skill 目录、加载状态与目录命令。
 * OUTPUT: 无结果计数的分类网格和短空态。
 * POS: 已安装 Skill 目录纯视图。
 */
import { Loader2 } from "lucide-react";

import { CapabilitySectionHeader } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { WORKSPACE_CATALOG_GRID_CLASS_NAME } from "@/shared/ui/layout/workspace-content-layout";
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
  const { t } = useI18n();

  if (loading) {
    return (
      <div className="flex min-h-80 items-center justify-center">
        <Loader2 className="h-5 w-5 animate-spin text-(--text-muted)" />
      </div>
    );
  }

  if (!groupedSkills.length) {
    return (
      <div className="flex min-h-48 items-center justify-center text-center">
        <p className="text-base font-medium text-(--text-default)">
          {t("capability.skills_empty_title")}
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {groupedSkills.map(([categoryName, items]) => (
        <section key={categoryName}>
          <CapabilitySectionHeader title={categoryName} />
          <div className={`${WORKSPACE_CATALOG_GRID_CLASS_NAME} gap-2.5`}>
            {items.map((skill: SkillInfo) => (
              <SkillsCard
                key={skill.name}
                busy={busySkillNames.has(skill.name)}
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
