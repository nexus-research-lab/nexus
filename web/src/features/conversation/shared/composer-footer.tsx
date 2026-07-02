import type { ReactNode, RefObject } from "react";
import { Paperclip, Plus, Repeat2, Target, X } from "lucide-react";

import { cn } from "@/lib/utils";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiActionMenu } from "@/shared/ui/action-menu";
import { LoadingOrb } from "@/shared/ui/feedback/loading-orb";
import { GlassSwitch } from "@/shared/ui/liquid-glass";
import { COMPOSER_FOOTER_CLASS_NAME } from "./composer-styles";

interface ComposerFooterProps {
  action_button_ref: RefObject<HTMLButtonElement | null>;
  active_error: string | null;
  can_create_goal: boolean;
  can_use_loop: boolean;
  can_stop_generation: boolean;
  char_count: number;
  goal_mode_extra: ReactNode;
  goal_scope_label: string;
  history_index: number;
  input_history_length: number;
  is_action_menu_open: boolean;
  is_dispatching: boolean;
  is_goal_creating: boolean;
  is_goal_mode: boolean;
  is_input_locked: boolean;
  is_near_limit: boolean;
  is_over_limit: boolean;
  is_preparing_attachments: boolean;
  max_length: number;
  on_action_menu_close: () => void;
  on_action_menu_toggle: () => void;
  on_attachment_select: () => void;
  on_cancel_goal: () => void;
  on_goal_toggle: (checked: boolean) => void;
  on_loop_select: () => void;
}

export function ComposerFooter({
  action_button_ref: actionButtonRef,
  active_error: activeError,
  can_create_goal: canCreateGoal,
  can_use_loop: canUseLoop,
  can_stop_generation: canStopGeneration,
  char_count: charCount,
  goal_mode_extra: goalModeExtra,
  goal_scope_label: goalScopeLabel,
  history_index: historyIndex,
  input_history_length: inputHistoryLength,
  is_action_menu_open: isActionMenuOpen,
  is_dispatching: isDispatching,
  is_goal_creating: isGoalCreating,
  is_goal_mode: isGoalMode,
  is_input_locked: isInputLocked,
  is_near_limit: isNearLimit,
  is_over_limit: isOverLimit,
  is_preparing_attachments: isPreparingAttachments,
  max_length: maxLength,
  on_action_menu_close: onActionMenuClose,
  on_action_menu_toggle: onActionMenuToggle,
  on_attachment_select: onAttachmentSelect,
  on_cancel_goal: onCancelGoal,
  on_goal_toggle: onGoalToggle,
  on_loop_select: onLoopSelect,
}: ComposerFooterProps) {
  const { t } = useI18n();

  return (
    <div className={COMPOSER_FOOTER_CLASS_NAME}>
      <div className="flex min-w-0 items-center gap-2 text-[10px] text-(--text-soft)">
        <div className="shrink-0">
          <button
            ref={actionButtonRef}
            aria-expanded={isActionMenuOpen}
            aria-haspopup="menu"
            aria-label={t("composer.open_actions")}
            className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
            disabled={isInputLocked}
            onClick={onActionMenuToggle}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
          <UiActionMenu
            anchor_ref={actionButtonRef}
            aria_label={t("composer.open_actions")}
            is_open={isActionMenuOpen}
            items={[
              {
                value: "attachment",
                label: t("composer.add_attachment"),
                icon: <Paperclip className="h-4 w-4 text-(--icon-muted)" />,
                disabled: isInputLocked || isPreparingAttachments || isGoalMode,
              },
              ...(canUseLoop
                ? [{
                    value: "loop",
                    label: t("composer.insert_loop"),
                    icon: <Repeat2 className="h-4 w-4 text-(--icon-muted)" />,
                    disabled: isInputLocked,
                  }]
                : []),
              {
                value: "goal",
                label: t("composer.start_goal"),
                icon: <Target className="h-4 w-4 text-(--primary)" />,
                trailing: (
                  <span
                    onClick={(event) => event.stopPropagation()}
                    onKeyDown={(event) => event.stopPropagation()}
                    role="presentation"
                  >
                    <GlassSwitch
                      checked={isGoalMode}
                      disabled={!canCreateGoal || isInputLocked || isGoalCreating}
                      on_change={onGoalToggle}
                      size="xs"
                    />
                  </span>
                ),
                active: isGoalMode,
                disabled: !canCreateGoal || isInputLocked || isGoalCreating,
                tone: "primary",
              },
            ]}
            placement="top"
            on_close={onActionMenuClose}
            on_select={(value) => {
              if (value === "attachment") {
                onAttachmentSelect();
                return;
              }
              if (value === "loop") {
                onLoopSelect();
                return;
              }
              if (value === "goal") {
                onGoalToggle(!isGoalMode);
              }
            }}
          />
        </div>

        {isGoalMode ? (
          <span className="inline-flex min-w-0 items-center gap-1.5 font-semibold text-(--primary)">
            <Target className="h-3.5 w-3.5 shrink-0" />
            <span>{t("composer.goal_mode")}</span>
            <span className="truncate font-medium text-(--text-muted)">{goalScopeLabel}</span>
            {goalModeExtra}
            <button
              aria-label={t("composer.cancel_goal_mode")}
              className="pointer-events-auto inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-[5px] text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              disabled={isGoalCreating}
              onClick={onCancelGoal}
              type="button"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ) : null}

        {isDispatching ? (
          <span className="flex items-center gap-2 text-(--success)">
            <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
            <span className="animate-pulse">{t("status.sending")}</span>
          </span>
        ) : canStopGeneration ? (
          <span className="flex items-center gap-2 text-(--success)">
            <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
            <span className="animate-pulse">{t("status.replying")}…</span>
            <span className="text-(--text-soft)">[{t("composer.esc_stop")}]</span>
          </span>
        ) : isPreparingAttachments ? (
          <span className="flex items-center gap-2 text-(--text-default)">
            <LoadingOrb frames={["·", "◦", "•", "◦"]} />
            <span>{t("composer.preparing_attachments")}</span>
          </span>
        ) : isGoalCreating ? (
          <span className="flex items-center gap-2 text-(--primary)">
            <LoadingOrb frames={["·", "◦", "•", "◦"]} />
            <span className="animate-pulse">{t("composer.goal_normalizing")}</span>
          </span>
        ) : activeError ? (
          <span className="text-(--destructive)">{activeError}</span>
        ) : null}
      </div>

      <div className="flex items-center gap-3 text-[10px] tabular-nums">
        {charCount > 0 ? (
          <div>
            <span
              className={cn(
                isOverLimit && "text-destructive",
                isNearLimit && !isOverLimit && "text-warning",
                !isNearLimit && "text-(--text-soft)",
              )}
            >
              {charCount}
            </span>
            <span className="text-(--text-soft)">/{maxLength}</span>
          </div>
        ) : null}
        {historyIndex >= 0 ? (
          <div className="text-[10px] text-(--text-default)">
            {t("composer.history_position", {
              current: historyIndex + 1,
              total: inputHistoryLength,
            })}
          </div>
        ) : null}
      </div>
    </div>
  );
}
