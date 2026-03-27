import { useParams } from "react-router-dom";

import { SkillsDirectory } from "@/features/skills/skills-directory";
import { AppStage } from "@/shared/ui/app-stage";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";
import { SkillsRouteParams } from "@/types/route";

export function SkillsPage() {
  const params = useParams<SkillsRouteParams>();

  return (
    <AppStage active_rail_item="skills">
      <WorkspacePageFrame content_padding_class_name="p-0">
        <SkillsDirectory selected_skill_name={params.skill_name} />
      </WorkspacePageFrame>
    </AppStage>
  );
}
