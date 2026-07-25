import { getUiChoiceClassName } from "@/shared/ui/form/choice-styles";

export const PICKER_TRIGGER_CLASS_NAME =
  "flex w-full items-center justify-between gap-3 radius-control-md border border-(--surface-control-border) bg-(--surface-control-field-background) px-5 py-4 text-left text-md font-medium text-(--text-strong) shadow-(--surface-control-field-shadow) transition-[border-color,background-color,box-shadow] duration-(--motion-duration-fast) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background)";

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
