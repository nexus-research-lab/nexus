/**
 * INPUT: 设置项标题、说明、选项和当前值。
 * OUTPUT: 统一的设置卡片、行、控件与响应式信息层级样式。
 * POS: 设置域共享视图原语；行级说明在窄屏仍用于解释选项影响。
 */
"use client";

import { cn } from "@/shared/ui/class-name";

export const SETTINGS_SECTION_TITLE_CLASS_NAME = "px-1 text-md font-semibold tracking-tight text-(--text-strong)";
export const SETTINGS_CARD_CLASS_NAME = "overflow-hidden rounded-[12px] border border-(--divider-subtle-color) bg-transparent";
export const SETTINGS_ROW_CLASS_NAME = "grid gap-3 px-4 py-3 md:grid-cols-[minmax(0,1fr)_minmax(180px,220px)] md:items-center";
export const SETTINGS_TEXT_ROW_CLASS_NAME = "flex min-w-0 items-start gap-3";
export const SETTINGS_ICON_CLASS_NAME = "flex h-7 w-7 shrink-0 items-center justify-center radius-control-lg bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary";
export const SETTINGS_ITEM_TITLE_CLASS_NAME = "text-base font-semibold tracking-tight text-(--text-strong)";
export const SETTINGS_ITEM_DESCRIPTION_CLASS_NAME = "mt-1 max-w-[520px] text-compact leading-5 text-(--text-soft)";
export const SETTINGS_CONTROL_LABEL_CLASS_NAME = "text-xs font-medium text-(--text-soft)";
export const SETTINGS_CONTROL_HEIGHT_CLASS_NAME = "h-7";
export const SETTINGS_CONTROL_TEXT_CLASS_NAME = "text-xs font-semibold leading-none";
export const SETTINGS_SELECT_BUTTON_CLASS_NAME = `${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} w-full rounded-[10px] border-(--divider-subtle-color) bg-transparent px-2.5 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} text-(--text-strong) shadow-none hover:border-(--divider-subtle-color) hover:bg-(--surface-interactive-hover-background) focus-visible:ring-0`;

interface SettingsSegmentedControlOption<T extends string> {
  label: string;
  value: T;
}

interface SettingsSegmentedControlProps<T extends string> {
  ariaLabel: string;
  disabled?: boolean;
  onChange: (value: T) => void;
  options: ReadonlyArray<SettingsSegmentedControlOption<T>>;
  value: T;
}

export function SettingsSegmentedControl<T extends string>({
  ariaLabel: ariaLabel,
  disabled,
  onChange: onChange,
  options,
  value,
}: SettingsSegmentedControlProps<T>) {
  return (
    <div
      aria-label={ariaLabel}
      className={`${SETTINGS_CONTROL_HEIGHT_CLASS_NAME} inline-flex w-full items-center surface-radius-md border border-(--divider-subtle-color) bg-transparent p-0.5`}
      role="group"
    >
      {options.map((option) => {
        const active = value === option.value;
        return (
          <button
            key={option.value}
            aria-pressed={active}
            className={cn(
              `inline-flex h-6 min-w-0 flex-1 items-center justify-center radius-control-sm px-2 ${SETTINGS_CONTROL_TEXT_CLASS_NAME} transition-colors`,
              active
                ? "bg-(--surface-interactive-active-background) text-(--text-strong) shadow-sm"
                : "text-(--text-soft) hover:text-(--text-default)",
            )}
            disabled={disabled}
            onClick={() => onChange(option.value)}
            type="button"
          >
            {option.label}
          </button>
        );
      })}
    </div>
  );
}
