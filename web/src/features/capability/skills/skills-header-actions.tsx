// INPUT: Skill 导入、更新检查、来源管理动作及其忙碌状态。
// OUTPUT: 桌面常驻导入并把检查更新、来源收进更多菜单；窄窗三项都进入同一菜单；等待状态复用统一 Spinner。
// POS: Skill 目录页头动作视图；不持有导入、更新或来源命令生命周期。
import {
  Download,
  Loader2,
  MoreHorizontal,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";
import { useRef, useState } from "react";

import { SKILLS_TOUR_ANCHORS } from "@/features/onboarding/tours/skills-tour";
import { useMediaQuery } from "@/shared/lib/react/use-media-query";
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
}

export function SkillsHeaderActions(props: SkillsHeaderActionsProps) {
  const isCompactLayout = useMediaQuery(APP_NARROW_VIEWPORT_MEDIA_QUERY);
  return isCompactLayout
    ? <SkillsHeaderCompactActions {...props} />
    : <SkillsHeaderDesktopActions {...props} />;
}

function useSkillsHeaderMenuItems({
  checkingUpdates,
  importing,
  includeImport,
}: Pick<SkillsHeaderActionsProps, "checkingUpdates" | "importing"> & { includeImport: boolean }): UiActionMenuItem[] {
  const { t } = useI18n();
  return [
    ...(includeImport ? [{
      disabled: importing,
      icon: importing
        ? <Loader2 className={getUiSpinnerClassName({ size: "md", tone: "muted" })} />
        : <Download className="h-4 w-4 text-(--icon-muted)" />,
      label: importing
        ? t("capability.skills_importing")
        : t("capability.import_skill"),
      tone: "primary" as const,
      value: "import",
    }] : []),
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
  ];
}

function SkillsHeaderMoreMenu({
  busy,
  items,
  onOpenImport,
  onCheckUpdates,
  onOpenSources,
  size,
  tourAnchor,
}: Pick<SkillsHeaderActionsProps, "onCheckUpdates" | "onOpenImport" | "onOpenSources"> & {
  busy: boolean;
  items: UiActionMenuItem[];
  size: "lg" | "md";
  tourAnchor: string;
}) {
  const { t } = useI18n();
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [isOpen, setIsOpen] = useState(false);
  return (
    <>
      <UiIconButton
        ref={buttonRef}
        aria-busy={busy || undefined}
        aria-expanded={isOpen}
        aria-haspopup="menu"
        aria-label={t("common.more_actions")}
        data-tour-anchor={tourAnchor}
        onClick={() => setIsOpen((current) => !current)}
        size={size}
        title={t("common.more_actions")}
      >
        {busy
          ? <Loader2 className={getUiSpinnerClassName({ size: "sm", tone: "muted" })} />
          : <MoreHorizontal className="h-4 w-4" />}
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

function SkillsHeaderCompactActions(props: SkillsHeaderActionsProps) {
  const items = useSkillsHeaderMenuItems({ ...props, includeImport: true });
  return (
    <SkillsHeaderMoreMenu
      {...props}
      busy={props.checkingUpdates || props.importing}
      items={items}
      size="lg"
      tourAnchor={SKILLS_TOUR_ANCHORS.import_skill}
    />
  );
}

/** 桌面只常驻“导入”；检查更新与来源管理是低频动作，收进同一个更多菜单。 */
function SkillsHeaderDesktopActions(props: SkillsHeaderActionsProps) {
  const { importing, onOpenImport } = props;
  const { t } = useI18n();
  const items = useSkillsHeaderMenuItems({ ...props, includeImport: false });
  return (
    <div className="flex items-center gap-1">
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
      <SkillsHeaderMoreMenu
        {...props}
        busy={props.checkingUpdates}
        items={items}
        size="md"
        tourAnchor={SKILLS_TOUR_ANCHORS.update_library}
      />
    </div>
  );
}
