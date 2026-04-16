/**
 * =====================================================
 * @File   : picker-styles.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

export const PICKER_TRIGGER_CLASS_NAME =
  "flex w-full items-center justify-between gap-3 rounded-[22px] border border-(--divider-subtle-color) bg-white/55 px-5 py-4 text-left text-[17px] font-medium text-(--text-strong) shadow-[0_10px_30px_rgba(125,145,189,0.08)] transition-[border-color,box-shadow] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--primary)_26%,var(--divider-subtle-color))]";

export const PICKER_POPOVER_CLASS_NAME =
  "fixed left-0 top-0 z-[10020] w-[min(480px,calc(100vw-96px))] rounded-[20px] border p-3 shadow-[0_20px_48px_rgba(66,82,104,0.22)]";

const PICKER_COLUMN_BUTTON_CLASS_NAME =
  "flex h-10 items-center justify-center rounded-[10px] px-3 text-[17px] font-semibold text-(--text-default) transition-[background,color,transform] duration-(--motion-duration-fast) hover:bg-[rgba(226,234,247,0.9)]";

export function get_picker_column_button_class_name(is_active: boolean): string {
  return [
    PICKER_COLUMN_BUTTON_CLASS_NAME,
    is_active
      ? "bg-[rgba(70,114,255,0.96)] text-white shadow-[0_8px_18px_rgba(70,114,255,0.22)] hover:text-[rgba(14,24,38,0.92)]"
      : "",
  ].join(" ");
}
