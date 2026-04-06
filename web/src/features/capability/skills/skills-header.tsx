import { Download, FolderUp, Puzzle, RefreshCw } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspacePillButton } from "@/shared/ui/workspace/workspace-pill-button";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/workspace-surface-header";

import type { SkillMarketplaceController } from "@/hooks/use-skill-marketplace";

interface SkillsHeaderProps {
  ctrl: SkillMarketplaceController;
}

export function SkillsHeader({ ctrl }: SkillsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.skills_badge", { count: ctrl.catalog_count })}
      density="compact"
      leading={<Puzzle className="h-4 w-4" />}
      subtitle={t("capability.skills_subtitle")}
      title={t("capability.skills_title")}
      trailing={
        <div className="flex items-center gap-2">
          <WorkspacePillButton density="compact" onClick={() => ctrl.file_input_ref.current?.click()} size="sm" variant="outlined">
            <FolderUp className="h-3.5 w-3.5" />
            {t("capability.import_local")}
          </WorkspacePillButton>
          <WorkspacePillButton density="compact" onClick={() => ctrl.set_git_prompt_open(true)} size="sm" variant="outlined">
            <Download className="h-3.5 w-3.5" />
            {t("capability.git_import")}
          </WorkspacePillButton>
          <WorkspacePillButton density="compact" onClick={() => void ctrl.handle_update_installed()} size="sm" variant="primary">
            <RefreshCw className="h-3.5 w-3.5" />
            {t("capability.update_library")}
          </WorkspacePillButton>
        </div>
      }
    />
  );
}
