// INPUT: Skill 导入、更新检查、来源管理与导览动作及其忙碌状态。
// OUTPUT: 桌面工具栏或窄窗共享动作菜单，并以统一 Spinner 投影等待状态。
// POS: Skill 目录页头动作视图；不持有导入、更新或来源命令生命周期。
import {
  Compass,
  Download,
  Loader2,
  MoreHorizontal,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";
import { useRef, useState } from "react";

import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";
import { useMediaQuery } from "@/hooks/ui/use-media-query";
import { APP_NARROW_VIEWPORT_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton, UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";

import type { SkillImportDialogMode } from "./controller/skill-marketplace-controller";

interface SkillsHeaderActionsProps {
  checkingUpdates: boolean;
  importing: boolean;
  onCheckUpdates: () => void;
  onOpenImport: (mode: SkillImportDialogMode) => void;
  onOpenSources: () => void;
  onReplayTour?: () => void;
}

export function SkillsHeaderActions(props: SkillsHeaderActionsProps) {
  const isCompactLayout = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  return isCompactLayout
    ? <SkillsHeaderCompactActions {...props} />
    : <SkillsHeaderDesktopActions {...props} />;
}

function SkillsHeaderCompactActions({
  checkingUpdates,
  importing,
  onCheckUpdates,
  onOpenImport,
  onOpenSources,
  onReplayTour,
}: SkillsHeaderActionsProps) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  const items: UiActionMenuItem[] = [
    {
      disabled: importing,
      icon: importing
        ? <Loader2 className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />
        : <Download className="h-4 w-4 text-(--icon-muted)" />,
      label: importing
        ? t("capability.skills_importing")
        : t("capability.import_skill"),
      tone: "primary",
      value: "import",
    },
    {
      disabled: checkingUpdates,
      icon: checkingUpdates
        ? <Loader2 className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />
        : <RefreshCw className="h-4 w-4 text-(--icon-muted)" />,
      label: checkingUpdates
        ? t("capability.skills_checking")
        : t("capability.update_library"),
      value: "updates",
    },
    {
      icon: <SlidersHorizontal className="h-4 w-4 text-(--icon-muted)" />,
      label: t("capability.skill_sources"),
      value: "sources",
    },
    ...(onReplayTour ? [{
      icon: <Compass className="h-4 w-4 text-(--icon-muted)" />,
      label: t("common.view_guide"),
      value: "guide",
    }] : []),
  ];

  return (
    <>
      <UiIconButton
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}
        onClick={() => setIsOpen((current) => !current)}
        size="lg"
        title={t("common.more_actions")}
      >
        <MoreHorizontal className="h-4 w-4" />
      </UiIconButton>
      <UiActionMenu
        anchorRef={buttonRef}
        ariaLabel={t("common.more_actions")}
        isOpen={isOpen}
        items={items}
        minWidth={190}
        onClose={() => setIsOpen(false)}
        onSelect={(value) => {
          const actions: Record<string, () => void> = {
            guide: () => onReplayTour?.(),
            import: () => onOpenImport("local"),
            sources: onOpenSources,
            updates: onCheckUpdates,
          };
          actions[value]?.();
        }}
      />
    </>
  );
}

function SkillsHeaderDesktopActions({
  checkingUpdates,
  importing,
  onCheckUpdates,
  onOpenImport,
  onOpenSources,
  onReplayTour,
}: SkillsHeaderActionsProps) {
  const { t } = useI18n();
  return (
    <div className="flex items-center gap-2">
      <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}>
        <UiButton
          disabled={importing}
          onClick={() => onOpenImport("local")}
          size="2xs"
          variant="text"
        >
          <Download className="h-3.5 w-3.5" />
          {importing
            ? t("capability.skills_importing")
            : t("capability.import_skill")}
        </UiButton>
      </div>
      <div className="flex items-center" data-tour-anchor={SKILLS_TOUR_ANCHORS.update_library}>
        <UiButton
          disabled={checkingUpdates}
          onClick={onCheckUpdates}
          size="2xs"
          variant="text"
        >
          {checkingUpdates ? (
            <Loader2 className={getUiSpinnerClassName({ size: "sm" })} />
          ) : (
            <RefreshCw className="h-3.5 w-3.5" />
          )}
          {checkingUpdates
            ? t("capability.skills_checking")
            : t("capability.update_library")}
        </UiButton>
      </div>
      <UiButton onClick={onOpenSources} size="2xs" variant="text">
        <SlidersHorizontal className="h-3.5 w-3.5" />
        {t("capability.skill_sources")}
      </UiButton>
      {onReplayTour ? (
        <UiButton onClick={onReplayTour} size="2xs" variant="text">
          <Compass className="h-3.5 w-3.5" />
          {t("common.view_guide")}
        </UiButton>
      ) : null}
    </div>
  );
}
