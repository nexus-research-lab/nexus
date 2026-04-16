import { Download, FolderUp, Puzzle, RefreshCw } from "lucide-react";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";

import type { DiscoveryMode, SkillMarketplaceController } from "@/hooks/capability/use-skill-marketplace";

const DISCOVERY_OPTIONS: { key: DiscoveryMode; label: string }[] = [
  { key: "catalog", label: "库内技能" },
  { key: "external", label: "社区技能" },
];

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
      title={t("capability.skills_title")}
      tabs={DISCOVERY_OPTIONS}
      active_tab={ctrl.discovery_mode}
      on_change_tab={ctrl.set_discovery_mode}
      trailing={
        <div className="flex items-center gap-2">
          <WorkspaceSurfaceToolbarAction onClick={() => ctrl.file_input_ref.current?.click()}>
            <FolderUp className="h-3.5 w-3.5" />
            {t("capability.import_local")}
          </WorkspaceSurfaceToolbarAction>
          <WorkspaceSurfaceToolbarAction onClick={() => ctrl.set_git_prompt_open(true)}>
            <Download className="h-3.5 w-3.5" />
            {t("capability.git_import")}
          </WorkspaceSurfaceToolbarAction>
          <WorkspaceSurfaceToolbarAction onClick={() => void ctrl.handle_update_installed()} tone="primary">
            <RefreshCw className="h-3.5 w-3.5" />
            {t("capability.update_library")}
          </WorkspaceSurfaceToolbarAction>
        </div>
      }
    />
  );
}
