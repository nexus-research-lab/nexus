import type { SkillInfo } from "@/types/capability/skill";

import { SkillDetailView } from "./skill-detail-view";
import { useSkillDetailController } from "./use-skill-detail-controller";

interface SkillDetailRouteProps {
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  onBack: () => void;
  onDeleted: () => Promise<void> | void;
  skillName: string;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export function SkillDetailRoute({
  deleteSkill,
  onBack,
  onDeleted,
  skillName,
  updateSkill,
}: SkillDetailRouteProps) {
  const controller = useSkillDetailController({
    deleteSkill,
    onDeleted,
    skillName,
    updateSkill,
  });

  return (
    <SkillDetailView
      activeAction={controller.activeAction}
      onBack={onBack}
      onDelete={() => void controller.deleteSkill()}
      onUpdate={() => void controller.updateSkill()}
      snapshot={controller.snapshot}
    />
  );
}
