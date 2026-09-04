// INPUT: Choice 的选择状态、尺寸、形状、tone 与视觉 variant。
// OUTPUT: 不改变几何且不依赖阴影表达选中的样式投影。
// POS: Choice 视觉状态真相；不渲染 DOM 或管理选项集合。

import { cn } from "@/shared/ui/class-name";

export type UiChoiceTone = "primary" | "neutral" | "danger" | "success";
export type UiChoiceVariant = "surface" | "picker" | "calendar" | "icon";
export type UiChoiceSize = "xs" | "sm" | "md" | "lg";
export type UiChoiceShape = "rounded" | "pill";

interface UiChoiceStyleOptions {
  active?: boolean;
  disabled?: boolean;
  muted?: boolean;
  shape?: UiChoiceShape;
  size?: UiChoiceSize;
  tone?: UiChoiceTone;
  variant?: UiChoiceVariant;
}

type ResolvedUiChoiceStyleOptions = Required<UiChoiceStyleOptions>;
type ChoiceClassList = Array<string | false>;
type ChoiceVariantClassResolver = (
  options: ResolvedUiChoiceStyleOptions,
) => ChoiceClassList;

const CHOICE_BASE_CLASS_NAME =
  "inline-flex items-center justify-center gap-1.5 border font-semibold transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]";

const SURFACE_CHOICE_SIZE_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "min-h-7 px-2 py-1 text-xs",
  sm: "min-h-8 px-2.5 py-1.5 text-compact",
  md: "min-h-9 px-3 py-2 text-compact",
  lg: "min-h-10 px-3.5 py-2.5 text-sm",
};

const SURFACE_CHOICE_ROUNDED_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "radius-control-xs",
  sm: "radius-control-sm",
  md: "radius-control-md",
  lg: "radius-control-lg",
};

const CHOICE_ACTIVE_CLASS_MAP: Record<UiChoiceTone, string> = {
  primary:
    "border-[color:color-mix(in_srgb,var(--primary)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)",
  neutral:
    "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background) text-(--text-strong)",
  danger:
    "border-[color:color-mix(in_srgb,var(--destructive)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_9%,transparent)] text-(--destructive)",
  success:
    "border-[color:color-mix(in_srgb,var(--success)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
};

const CHOICE_INACTIVE_CLASS_NAME =
  "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)";

const PICKER_CHOICE_BASE_CLASS_NAME =
  "flex h-10 items-center justify-center radius-control-md border px-3 text-md font-semibold transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-40";

const PICKER_CHOICE_ACTIVE_CLASS_NAME =
  "border-[color:color-mix(in_srgb,var(--primary)_58%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_88%,white)] text-white";

const PICKER_CHOICE_INACTIVE_CLASS_NAME =
  "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background)";

const CALENDAR_CHOICE_BASE_CLASS_NAME =
  "flex h-8 items-center justify-center radius-control-md border text-xs font-semibold transition-[background,border-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-40";

const CALENDAR_CHOICE_ACTIVE_CLASS_NAME =
  "border-[color:color-mix(in_srgb,var(--primary)_58%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_90%,white)] text-white";

const CALENDAR_CHOICE_INACTIVE_CLASS_NAME =
  "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background)";

const ICON_CHOICE_BASE_CLASS_NAME =
  "relative inline-flex cursor-pointer items-center justify-center overflow-hidden border p-0 transition-[background,border-color,color] duration-(--motion-duration-fast) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

const ICON_CHOICE_SIZE_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "h-7 w-7 radius-control-sm",
  sm: "h-8 w-8 radius-control-md",
  md: "h-10 w-10 radius-control-lg",
  lg: "h-12 w-12 surface-radius-md",
};

const ICON_CHOICE_ACTIVE_CLASS_NAME =
  "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background)";

const ICON_CHOICE_INACTIVE_CLASS_NAME =
  "border-(--surface-inset-border) bg-transparent hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)";

const CHOICE_VARIANT_CLASS_RESOLVER: Record<
  UiChoiceVariant,
  ChoiceVariantClassResolver
> = {
  calendar: resolveCalendarChoiceClasses,
  icon: resolveIconChoiceClasses,
  picker: resolvePickerChoiceClasses,
  surface: resolveSurfaceChoiceClasses,
};

export function getUiChoiceClassName(
  options: UiChoiceStyleOptions,
  className?: string,
): string {
  const resolved = resolveChoiceStyleOptions(options);
  return cn(...CHOICE_VARIANT_CLASS_RESOLVER[resolved.variant](resolved), className);
}

function resolveChoiceStyleOptions(
  options: UiChoiceStyleOptions,
): ResolvedUiChoiceStyleOptions {
  return {
    active: optionOrDefault(options.active, false),
    disabled: optionOrDefault(options.disabled, false),
    muted: optionOrDefault(options.muted, false),
    shape: optionOrDefault(options.shape, "rounded"),
    size: optionOrDefault(options.size, "md"),
    tone: optionOrDefault(options.tone, "primary"),
    variant: optionOrDefault(options.variant, "surface"),
  };
}

function resolveSurfaceChoiceClasses({
  active,
  disabled,
  shape,
  size,
  tone,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    CHOICE_BASE_CLASS_NAME,
    SURFACE_CHOICE_SIZE_CLASS_MAP[size],
    shape === "pill" ? "rounded-full" : SURFACE_CHOICE_ROUNDED_CLASS_MAP[size],
    active ? CHOICE_ACTIVE_CLASS_MAP[tone] : CHOICE_INACTIVE_CLASS_NAME,
    disabled && "pointer-events-none",
  ];
}

function resolvePickerChoiceClasses({
  active,
  disabled,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    PICKER_CHOICE_BASE_CLASS_NAME,
    active ? PICKER_CHOICE_ACTIVE_CLASS_NAME : PICKER_CHOICE_INACTIVE_CLASS_NAME,
    disabled && "pointer-events-none",
  ];
}

function resolveCalendarChoiceClasses({
  active,
  disabled,
  muted,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    CALENDAR_CHOICE_BASE_CLASS_NAME,
    active ? CALENDAR_CHOICE_ACTIVE_CLASS_NAME : CALENDAR_CHOICE_INACTIVE_CLASS_NAME,
    muted && !active && "text-(--text-soft)",
    disabled && "pointer-events-none text-(--text-soft)",
  ];
}

function resolveIconChoiceClasses({
  active,
  disabled,
  size,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    ICON_CHOICE_BASE_CLASS_NAME,
    ICON_CHOICE_SIZE_CLASS_MAP[size],
    active ? ICON_CHOICE_ACTIVE_CLASS_NAME : ICON_CHOICE_INACTIVE_CLASS_NAME,
    disabled && "pointer-events-none",
  ];
}

function optionOrDefault<T>(value: T | undefined, fallback: T): T {
  return value === undefined ? fallback : value;
}
