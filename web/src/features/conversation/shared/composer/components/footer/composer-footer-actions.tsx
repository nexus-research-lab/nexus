import type { ReactNode, RefObject } from "react";
import { Paperclip, Plus, Repeat2, Target } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import {
  UiActionMenu,
  type UiActionMenuItem,
} from "@/shared/ui/menu/action-menu";
import { GlassSwitch } from "@/shared/ui/liquid-glass/glass-switch";

type ComposerActionValue = "attachment" | "goal" | "loop";

interface ComposerFooterActionsProps {
  actionButtonRef: RefObject<HTMLButtonElement | null>;
  canCreateGoal: boolean;
  canUseLoop: boolean;
  isActionMenuOpen: boolean;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  onActionMenuClose: () => void;
  onActionMenuToggle: () => void;
  onAttachmentSelect: () => void;
  onGoalToggle: (checked: boolean) => void;
  onLoopSelect: () => void;
}

interface VisibleActionItem {
  item: UiActionMenuItem;
  visible: boolean;
}

export function ComposerFooterActions({
  actionButtonRef,
  canCreateGoal,
  canUseLoop,
  isActionMenuOpen,
  isGoalCreating,
  isGoalMode,
  isPreparingAttachments,
  onActionMenuClose,
  onActionMenuToggle,
  onAttachmentSelect,
  onGoalToggle,
  onLoopSelect,
}: ComposerFooterActionsProps) {
  const { t } = useI18n();
  const items = buildActionItems({
    canCreateGoal,
    canUseLoop,
    goalSwitch: (
      <span
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => event.stopPropagation()}
        role="presentation"
      >
        <GlassSwitch
          checked={isGoalMode}
          disabled={!canCreateGoal || isGoalCreating}
          onChange={onGoalToggle}
          size="xs"
        />
      </span>
    ),
    isGoalCreating,
    isGoalMode,
    isPreparingAttachments,
    labels: {
      attachment: t("composer.add_attachment"),
      goal: t("composer.start_goal"),
      loop: t("composer.insert_loop"),
    },
  });
  const commands = new Map<string, () => void>([
    ["attachment", onAttachmentSelect],
    ["loop", onLoopSelect],
    ["goal", () => onGoalToggle(!isGoalMode)],
  ]);

  return (
    <div className="shrink-0">
      <button
        ref={actionButtonRef}
        aria-expanded={isActionMenuOpen}
        aria-haspopup="menu"
        aria-label={t("composer.open_actions")}
        className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
        onClick={onActionMenuToggle}
        type="button"
      >
        <Plus className="h-4 w-4" />
      </button>
      <UiActionMenu
        anchorRef={actionButtonRef}
        ariaLabel={t("composer.open_actions")}
        isOpen={isActionMenuOpen}
        items={items}
        onClose={onActionMenuClose}
        onSelect={(value) => commands.get(value)?.()}
        placement="top"
      />
    </div>
  );
}

function buildActionItems({
  canCreateGoal,
  canUseLoop,
  goalSwitch,
  isGoalCreating,
  isGoalMode,
  isPreparingAttachments,
  labels,
}: {
  canCreateGoal: boolean;
  canUseLoop: boolean;
  goalSwitch: ReactNode;
  isGoalCreating: boolean;
  isGoalMode: boolean;
  isPreparingAttachments: boolean;
  labels: Record<ComposerActionValue, string>;
}): UiActionMenuItem[] {
  const candidates: VisibleActionItem[] = [
    {
      item: {
        disabled: isGoalMode || isPreparingAttachments,
        icon: <Paperclip className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.attachment,
        value: "attachment",
      },
      visible: true,
    },
    {
      item: {
        icon: <Repeat2 className="h-4 w-4 text-(--icon-muted)" />,
        label: labels.loop,
        value: "loop",
      },
      visible: canUseLoop,
    },
    {
      item: {
        active: isGoalMode,
        disabled: !canCreateGoal || isGoalCreating,
        icon: <Target className="h-4 w-4 text-(--primary)" />,
        label: labels.goal,
        tone: "primary",
        trailing: goalSwitch,
        value: "goal",
      },
      visible: true,
    },
  ];
  return candidates
    .filter((candidate) => candidate.visible)
    .map((candidate) => candidate.item);
}
