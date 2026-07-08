/**
 * =====================================================
 * @File   : picker-styles.ts
 * @Date   : 2026-04-16 14:28
 * @Author : leemysw
 * 2026-04-16 14:28   Create
 * =====================================================
 */

import { getUiChoiceClassName } from "@/shared/ui/choice-styles";

export const PICKER_TRIGGER_CLASS_NAME =
  "flex w-full items-center justify-between gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-5 py-4 text-left text-[17px] font-medium text-(--text-strong) transition-[border-color,background-color] duration-(--motion-duration-fast) hover:border-[color:color-mix(in_srgb,var(--primary)_26%,var(--divider-subtle-color))] hover:bg-(--surface-interactive-hover-background)";

export const PICKER_POPOVER_CLASS_NAME =
  "fixed left-0 top-0 z-[10020] w-[min(480px,calc(100vw-96px))] rounded-[12px] border p-3 shadow-[0_14px_32px_rgba(66,82,104,0.16)]";

export function getPickerColumnButtonClassName(isActive: boolean, isDisabled = false): string {
  return getUiChoiceClassName({
    active: isActive,
    disabled: isDisabled,
    variant: "picker",
  });
}

export function getPickerDateButtonClassName(
  isActive: boolean,
  options?: {
    disabled?: boolean;
    muted?: boolean;
  },
): string {
  return getUiChoiceClassName({
    active: isActive,
    disabled: options?.disabled,
    muted: options?.muted,
    variant: "calendar",
  });
}
