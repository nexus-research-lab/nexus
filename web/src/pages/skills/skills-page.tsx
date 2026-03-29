import { useParams } from "react-router-dom";

import { SkillsDirectory } from "@/features/skills/skills-directory";
import { SkillsDetailPage } from "@/features/skills/skills-detail-panel";
import { AppStage } from "@/shared/ui/app-stage";
import { WorkspacePageFrame } from "@/shared/ui/workspace-page-frame";
import { SkillsRouteParams } from "@/types/route";

/** Skills 页面 — 根据路由参数判断显示列表还是详情 */
export function SkillsPage() {
  const params = useParams<SkillsRouteParams>();

  return (
    <AppStage>
      <WorkspacePageFrame content_padding_class_name="p-0">
        {params.skill_name ? (
          <SkillsDetailPage skill_name={params.skill_name} />
        ) : (
          <SkillsDirectory />
        )}
      </WorkspacePageFrame>
    </AppStage>
  );
}
