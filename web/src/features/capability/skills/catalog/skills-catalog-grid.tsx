/**
 * INPUT: 已分组 Skill 目录、加载状态与目录命令。
 * OUTPUT: 无结果计数的分类网格和短空态。
 * POS: 已安装 Skill 目录纯视图。
 */
import { CapabilitySectionHeader } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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
      <UiResourceState
        className="min-h-80"
        size="md"
        state="loading"
        title={t("capability.skills_loading")}
        variant="plain"
      />
    );
  }

  if (!groupedSkills.length) {
    return (
      <UiResourceState
        className="min-h-48"
        description={t("capability.skills_empty_description")}
        size="md"
        state="empty"
        title={t("capability.skills_empty_title")}
        variant="plain"
      />
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
