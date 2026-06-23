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
  action_button_ref,
  active_error,
  can_create_goal,
  can_use_loop,
  can_stop_generation,
  char_count,
  goal_mode_extra,
  goal_scope_label,
  history_index,
  input_history_length,
  is_action_menu_open,
  is_dispatching,
  is_goal_creating,
  is_goal_mode,
  is_input_locked,
  is_near_limit,
  is_over_limit,
  is_preparing_attachments,
  max_length,
  on_action_menu_close,
  on_action_menu_toggle,
  on_attachment_select,
  on_cancel_goal,
  on_goal_toggle,
  on_loop_select,
}: ComposerFooterProps) {
  const { t } = useI18n();

  return (
    <div className={COMPOSER_FOOTER_CLASS_NAME}>
      <div className="flex min-w-0 items-center gap-2 text-[10px] text-(--text-soft)">
        <div className="shrink-0">
          <button
            ref={action_button_ref}
            aria-expanded={is_action_menu_open}
            aria-haspopup="menu"
            aria-label={t("composer.open_actions")}
            className="inline-flex h-6 w-6 items-center justify-center rounded-[8px] text-(--icon-default) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong) disabled:pointer-events-none disabled:opacity-(--disabled-opacity)"
            disabled={is_input_locked}
            onClick={on_action_menu_toggle}
            type="button"
          >
            <Plus className="h-4 w-4" />
          </button>
          <UiActionMenu
            anchor_ref={action_button_ref}
            aria_label={t("composer.open_actions")}
            is_open={is_action_menu_open}
            items={[
              {
                value: "attachment",
                label: t("composer.add_attachment"),
                icon: <Paperclip className="h-4 w-4 text-(--icon-muted)" />,
                disabled: is_input_locked || is_preparing_attachments || is_goal_mode,
              },
              ...(can_use_loop
                ? [{
                    value: "loop",
                    label: t("composer.insert_loop"),
                    icon: <Repeat2 className="h-4 w-4 text-(--icon-muted)" />,
                    disabled: is_input_locked,
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
                  >
                    <GlassSwitch
                      checked={is_goal_mode}
                      disabled={!can_create_goal || is_input_locked || is_goal_creating}
                      on_change={on_goal_toggle}
                      size="xs"
                    />
                  </span>
                ),
                active: is_goal_mode,
                disabled: !can_create_goal || is_input_locked || is_goal_creating,
                tone: "primary",
              },
            ]}
            placement="top"
            on_close={on_action_menu_close}
            on_select={(value) => {
              if (value === "attachment") {
                on_attachment_select();
                return;
              }
              if (value === "loop") {
                on_loop_select();
                return;
              }
              if (value === "goal") {
                on_goal_toggle(!is_goal_mode);
              }
            }}
          />
        </div>

        {is_goal_mode ? (
          <span className="inline-flex min-w-0 items-center gap-1.5 font-semibold text-(--primary)">
            <Target className="h-3.5 w-3.5 shrink-0" />
            <span>{t("composer.goal_mode")}</span>
            <span className="truncate font-medium text-(--text-muted)">{goal_scope_label}</span>
            {goal_mode_extra}
            <button
              aria-label={t("composer.cancel_goal_mode")}
              className="pointer-events-auto inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-[5px] text-(--text-soft) transition-colors hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
              disabled={is_goal_creating}
              onClick={on_cancel_goal}
              type="button"
            >
              <X className="h-3 w-3" />
            </button>
          </span>
        ) : null}

        {is_dispatching ? (
          <span className="flex items-center gap-2 text-(--success)">
            <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
            <span className="animate-pulse">{t("status.sending")}</span>
          </span>
        ) : can_stop_generation ? (
          <span className="flex items-center gap-2 text-(--success)">
            <LoadingOrb frames={["✽", "✻", "✶", "✢", "·"]} />
            <span className="animate-pulse">{t("status.replying")}…</span>
            <span className="text-(--text-soft)">[{t("composer.esc_stop")}]</span>
          </span>
        ) : is_preparing_attachments ? (
          <span className="flex items-center gap-2 text-(--text-default)">
            <LoadingOrb frames={["·", "◦", "•", "◦"]} />
            <span>{t("composer.preparing_attachments")}</span>
          </span>
        ) : is_goal_creating ? (
          <span className="flex items-center gap-2 text-(--primary)">
            <LoadingOrb frames={["·", "◦", "•", "◦"]} />
            <span className="animate-pulse">{t("composer.goal_normalizing")}</span>
          </span>
        ) : active_error ? (
          <span className="text-(--destructive)">{active_error}</span>
        ) : null}
      </div>

      <div className="flex items-center gap-3 text-[10px] tabular-nums">
        {char_count > 0 ? (
          <div>
            <span
              className={cn(
                is_over_limit && "text-destructive",
                is_near_limit && !is_over_limit && "text-warning",
                !is_near_limit && "text-(--text-soft)",
              )}
            >
              {char_count}
            </span>
            <span className="text-(--text-soft)">/{max_length}</span>
          </div>
        ) : null}
        {history_index >= 0 ? (
          <div className="text-[10px] text-(--text-default)">
            {t("composer.history_position", {
              current: history_index + 1,
              total: input_history_length,
            })}
          </div>
        ) : null}
      </div>
    </div>
  );
}
