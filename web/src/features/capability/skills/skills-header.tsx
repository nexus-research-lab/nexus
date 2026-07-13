import { Compass, Download, Loader2, Puzzle, RefreshCw, SlidersHorizontal } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import type { TranslationKey } from "@/shared/i18n/messages";
import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";

import type {
  DiscoveryMode,
  SkillImportDialogMode,
} from "./controller/skill-marketplace-controller";

const DISCOVERY_OPTIONS: { key: DiscoveryMode; labelKey: TranslationKey }[] = [
  { key: "catalog", labelKey: "capability.skills_tab_catalog" },
  { key: "external", labelKey: "capability.skills_tab_external" },
];

interface SkillsHeaderProps {
  catalogCount: number;
  checkingUpdates: boolean;
  discoveryMode: DiscoveryMode;
  importing: boolean;
  onChangeDiscoveryMode: (mode: DiscoveryMode) => void;
  onCheckUpdates: () => void;
  onOpenImport: (mode: SkillImportDialogMode) => void;
  onOpenSources: () => void;
  onReplayTour?: () => void;
}

export function SkillsHeader({
  catalogCount,
  checkingUpdates,
  discoveryMode,
  importing,
  onChangeDiscoveryMode,
  onCheckUpdates,
  onOpenImport,
  onOpenSources,
  onReplayTour,
}: SkillsHeaderProps) {
  const { t } = useI18n();

  return (
    <WorkspaceSurfaceHeader
      badge={t("capability.skills_badge", { count: catalogCount })}
      leading={<Puzzle className="h-4 w-4" />}
      title={t("capability.skills")}
      tabs={DISCOVERY_OPTIONS.map((item) => ({
        key: item.key,
        label: t(item.labelKey),
      }))}
      tabsNavAnchor={SKILLS_TOUR_ANCHORS.modes}
      activeTab={discoveryMode}
      onChangeTab={onChangeDiscoveryMode}
      trailing={
        <div className="flex items-center gap-2">
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}>
            <WorkspaceSurfaceToolbarAction
              disabled={importing}
              onClick={() => onOpenImport("local")}
            >
              <Download className="h-3.5 w-3.5" />
              {importing ? "导入中" : t("capability.import_skill")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.update_library}>
            <WorkspaceSurfaceToolbarAction
              disabled={checkingUpdates}
              onClick={onCheckUpdates}
            >
              {checkingUpdates ? (
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
              ) : (
                <RefreshCw className="h-3.5 w-3.5" />
              )}
              {checkingUpdates ? "检查中" : t("capability.update_library")}
            </WorkspaceSurfaceToolbarAction>
          </div>
          <div className="flex items-center">
            <WorkspaceSurfaceToolbarAction onClick={onOpenSources}>
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
