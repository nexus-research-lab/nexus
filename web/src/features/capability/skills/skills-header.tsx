import { Download, FolderUp, Puzzle, RefreshCw } from "lucide-react";

import { WorkspacePillButton } from "@/shared/ui/workspace-pill-button";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace-surface-header";

import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

interface SkillsHeaderProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsHeader({ ctrl }: SkillsHeaderProps) {
  return (
    <WorkspaceSurfaceHeader
      badge={`已安装 ${ctrl.installed_count}`}
      leading={<Puzzle className="h-4 w-4" />}
      subtitle="浏览和维护全局技能资源池"
      title="Skills"
      trailing={
        <div className="flex items-center gap-2">
          <WorkspacePillButton onClick={() => ctrl.file_input_ref.current?.click()} size="sm">
            <FolderUp className="h-3.5 w-3.5" />
            导入本地
          </WorkspacePillButton>
          <WorkspacePillButton onClick={() => ctrl.set_git_prompt_open(true)} size="sm">
            <Download className="h-3.5 w-3.5" />
            Git 安装
          </WorkspacePillButton>
          <WorkspacePillButton onClick={() => void ctrl.handle_update_installed()} size="sm">
            <RefreshCw className="h-3.5 w-3.5" />
            更新技能库
          </WorkspacePillButton>
        </div>
      }
    />
  );
}
