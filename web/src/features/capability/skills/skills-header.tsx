import { Compass, Download, Puzzle, RefreshCw, SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { TranslationKey } from "@/shared/i18n/messages";
import { SKILLS_TOUR_ANCHORS } from "./skills-tour";

import type { DiscoveryMode, SkillMarketplaceController } from "./skills-view-model";

const DISCOVERY_OPTIONS: { key: DiscoveryMode; label_key: TranslationKey }[] = [
  { key: "catalog", label_key: "capability.skills_tab_catalog" },
  { key: "external", label_key: "capability.skills_tab_external" },
];

interface SkillsHeaderProps {
  ctrl: SkillMarketplaceController;
  on_replay_tour?: () => void;
}

export function SkillsHeader({ ctrl, on_replay_tour }: SkillsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.skills_badge", { count: ctrl.catalog_count })}
      density="compact"
      leading={<Puzzle className="h-4 w-4" />}
      title={t("capability.skills")}
      tabs={DISCOVERY_OPTIONS.map((item) => ({
        key: item.key,
        label: t(item.label_key),
      }))}
      tabs_nav_anchor={SKILLS_TOUR_ANCHORS.modes}
      active_tab={ctrl.discovery_mode}
      on_change_tab={ctrl.set_discovery_mode}
      trailing={
        <div className="flex items-center gap-2">
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}>
            <WorkspaceSurfaceToolbarAction onClick={() => ctrl.set_import_dialog_mode("local")}>
              <Download className="h-3.5 w-3.5" />
              {t("capability.import_skill")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.update_library}>
            <WorkspaceSurfaceToolbarAction onClick={() => void ctrl.handle_update_installed()}>
              <RefreshCw className="h-3.5 w-3.5" />
              {t("capability.update_library")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center">
            <WorkspaceSurfaceToolbarAction onClick={() => ctrl.set_source_manager_open(true)}>
              <SlidersHorizontal className="h-3.5 w-3.5" />
              {t("capability.skill_sources")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          {on_replay_tour ? (
            <div className="flex items-center">
              <WorkspaceSurfaceToolbarAction onClick={on_replay_tour}>
                <Compass className="h-3.5 w-3.5" />
                {t("common.view_guide")}
              </WorkspaceSurfaceToolbarAction>
            </div>
          ) : null}
        </div>
      }
    />
  );
}
