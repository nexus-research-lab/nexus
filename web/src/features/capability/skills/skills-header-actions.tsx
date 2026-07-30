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
import { CONVERSATION_FOCUS_MEDIA_QUERY } from "@/lib/layout/home-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";

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
  const isCompactLayout = useMediaQuery(CONVERSATION_FOCUS_MEDIA_QUERY);
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
        ? <Loader2 className="h-4 w-4 animate-spin text-(--icon-muted)" />
        : <Download className="h-4 w-4 text-(--icon-muted)" />,
      label: importing ? "导入中" : t("capability.import_skill"),
      tone: "primary",
      value: "import",
    },
    {
      disabled: checkingUpdates,
      icon: checkingUpdates
        ? <Loader2 className="h-4 w-4 animate-spin text-(--icon-muted)" />
        : <RefreshCw className="h-4 w-4 text-(--icon-muted)" />,
      label: checkingUpdates ? "检查中" : t("capability.update_library"),
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
      <button
        ref={buttonRef}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        className="inline-flex h-9 w-9 items-center justify-center rounded-[8px] text-(--icon-default) transition hover:bg-(--interaction-hover-background) hover:text-(--text-strong)"
        data-tour-anchor={SKILLS_TOUR_ANCHORS.import_skill}
        onClick={() => setIsOpen((current) => !current)}
        title={t("common.more_actions")}
        type="button"
      >
        <MoreHorizontal className="h-4 w-4" />
      </button>
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
      <WorkspaceSurfaceToolbarAction onClick={onOpenSources}>
        <SlidersHorizontal className="h-3.5 w-3.5" />
        {t("capability.skill_sources")}
      </WorkspaceSurfaceToolbarAction>
      {onReplayTour ? (
        <WorkspaceSurfaceToolbarAction onClick={onReplayTour}>
          <Compass className="h-3.5 w-3.5" />
          {t("common.view_guide")}
        </WorkspaceSurfaceToolbarAction>
      ) : null}
    </div>
  );
}
