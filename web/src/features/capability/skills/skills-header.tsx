import { Puzzle } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import type { TranslationKey } from "@/shared/i18n/messages";
import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";

import { SkillsHeaderActions } from "./skills-header-actions";
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
        <SkillsHeaderActions
          checkingUpdates={checkingUpdates}
          importing={importing}
          onCheckUpdates={onCheckUpdates}
          onOpenImport={onOpenImport}
          onOpenSources={onOpenSources}
          onReplayTour={onReplayTour}
        />
      }
    />
  );
}
