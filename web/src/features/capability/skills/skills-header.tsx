import { Compass, Download, Puzzle, RefreshCw, SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { TranslationKey } from "@/shared/i18n/messages";
import { SKILLS_TOUR_ANCHORS } from "./skills-tour";

import type { DiscoveryMode, SkillMarketplaceController } from "./skills-view-model";

const DISCOVERY_OPTIONS: { key: DiscoveryMode; labelKey: TranslationKey }[] = [
  { key: "catalog", labelKey: "capability.skills_tab_catalog" },
  { key: "external", labelKey: "capability.skills_tab_external" },
];

interface SkillsHeaderProps {
  ctrl: SkillMarketplaceController;
  onReplayTour?: () => void;
}

export function SkillsHeader({ ctrl, onReplayTour: onReplayTour }: SkillsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.skills_badge", { count: ctrl.catalogCount })}
      density="compact"
      leading={<Puzzle className="h-4 w-4" />}
      title={t("capability.skills")}
      tabs={DISCOVERY_OPTIONS.map((item) => ({
        key: item.key,
        label: t(item.labelKey),
      }))}
      tabsNavAnchor={SKILLS_TOUR_ANCHORS.modes}
      activeTab={ctrl.discoveryMode}
      onChangeTab={ctrl.setDiscoveryMode}
      trailing={
        <div className="flex items-center gap-2">
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}>
            <WorkspaceSurfaceToolbarAction onClick={() => ctrl.setImportDialogMode("local")}>
              <Download className="h-3.5 w-3.5" />
              {t("capability.import_skill")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.update_library}>
            <WorkspaceSurfaceToolbarAction onClick={() => void ctrl.handleUpdateInstalled()}>
              <RefreshCw className="h-3.5 w-3.5" />
              {t("capability.update_library")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center">
            <WorkspaceSurfaceToolbarAction onClick={() => ctrl.setSourceManagerOpen(true)}>
              <SlidersHorizontal className="h-3.5 w-3.5" />
              {t("capability.skill_sources")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          {onReplayTour ? (
            <div className="flex items-center">
              <WorkspaceSurfaceToolbarAction onClick={onReplayTour}>
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
