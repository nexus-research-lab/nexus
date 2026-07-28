import type { SkillInfo } from "@/types/capability/skill";

import { SkillDetailView } from "./skill-detail-view";
import { useSkillDetailController } from "./use-skill-detail-controller";

interface SkillDetailRouteProps {
  deleteSkill: (skill: SkillInfo) => Promise<boolean>;
  onAgentBindingChanged: () => Promise<void> | void;
  onBack: () => void;
  onDeleted: () => Promise<void> | void;
  skillName: string;
  updateSkill: (skillName: string) => Promise<boolean>;
}

export function SkillDetailRoute({
  deleteSkill,
  onAgentBindingChanged,
  onBack,
  onDeleted,
  skillName,
  updateSkill,
}: SkillDetailRouteProps) {
  const controller = useSkillDetailController({
    deleteSkill,
    onAgentBindingChanged,
    onDeleted,
    skillName,
    updateSkill,
  });

  return (
    <SkillDetailView
      activeAction={controller.activeAction}
      agentBindings={controller.agentBindings}
      agentToggleError={controller.agentToggleError}
      agentsLoading={controller.agentsLoading}
      busyAgentId={controller.busyAgentId}
      onBack={onBack}
      onDelete={() => void controller.deleteSkill()}
      onAgentToggle={(binding) => void controller.toggleAgent(binding)}
      onUpdate={() => void controller.updateSkill()}
      snapshot={controller.snapshot}
    />
  );
}
