/**
 * =====================================================
 * @File   : picker-styles.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

import { get_ui_choice_class_name } from "@/shared/ui/choice-styles";

export const PICKER_TRIGGER_CLASS_NAME =
  "flex w-full items-center justify-between gap-3 rounded-[22px] border border-(--divider-subtle-color) bg-white/55 px-5 py-4 text-left text-[17px] font-medium text-(--text-strong) shadow-[0_10px_30px_rgba(125,145,189,0.08)] transition-[border-color,box-shadow] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--primary)_26%,var(--divider-subtle-color))]";

export const PICKER_POPOVER_CLASS_NAME =
  "fixed left-0 top-0 z-[10020] w-[min(480px,calc(100vw-96px))] rounded-[20px] border p-3 shadow-[0_20px_48px_rgba(66,82,104,0.22)]";

export function get_picker_column_button_class_name(is_active: boolean, is_disabled = false): string {
  return get_ui_choice_class_name({
    active: is_active,
    disabled: is_disabled,
    variant: "picker",
  });
}

export function get_picker_date_button_class_name(
  is_active: boolean,
  options?: {
    disabled?: boolean;
    muted?: boolean;
  },
): string {
  return get_ui_choice_class_name({
    active: is_active,
    disabled: options?.disabled,
    muted: options?.muted,
    variant: "calendar",
  });
}
