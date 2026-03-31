import { SkillsDirectory } from "@/features/skills/skills-directory";
import { WorkspacePageFrame } from "@/shared/ui/workspace/workspace-page-frame";

/** Skills 页面 — 卡片网格 + 弹窗详情（不再使用路由切换详情页） */
export function SkillsPage() {
  return (
    <WorkspacePageFrame content_padding_class_name="p-0">
      <SkillsDirectory />
    </WorkspacePageFrame>
  );
}
