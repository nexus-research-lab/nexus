// INPUT: Choice 的选择状态、尺寸、形状、tone 与视觉 variant。
// OUTPUT: 与普通控件配套的字号/高度、唯一焦点/禁用状态与按内容区分的选择材质。
// POS: Choice 视觉状态真相；原生 button/input 决定禁用，不用穿透命中区替代语义。

import { cn } from "@/shared/ui/class-name";

export type UiChoiceTone = "primary" | "neutral" | "danger" | "success";
export type UiChoiceVariant = "surface" | "picker" | "calendar" | "icon";
export type UiChoiceSize = "xs" | "sm" | "md" | "lg";
export type UiChoiceShape = "rounded" | "pill";

interface UiChoiceStyleOptions {
  active?: boolean;
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
  "inline-flex cursor-pointer items-center justify-center gap-2 border ui-type-weight-medium transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) has-[input:disabled]:cursor-not-allowed has-[input:disabled]:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] has-[input:focus-visible]:ring-2 has-[input:focus-visible]:ring-[color:var(--ring)]";

const SURFACE_CHOICE_SIZE_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "min-h-7 px-2 py-1 ui-type-metadata",
  sm: "min-h-8 px-2.5 py-1 ui-type-supporting",
  md: "min-h-9 px-3 py-1.5 ui-type-control",
  lg: "min-h-10 px-3.5 py-2 ui-type-control",
};

const SURFACE_CHOICE_ROUNDED_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "radius-control-xs",
  sm: "radius-control-sm",
  md: "radius-control-md",
  lg: "radius-control-lg",
};

const CHOICE_ACTIVE_CLASS_MAP: Record<UiChoiceTone, string> = {
  primary:
    "border-[color:color-mix(in_srgb,var(--primary)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--brand-action)",
  neutral:
    "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background) text-(--text-strong)",
  danger:
    "border-[color:color-mix(in_srgb,var(--destructive)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_9%,transparent)] text-(--destructive)",
  success:
    "border-[color:color-mix(in_srgb,var(--success)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
};

const CHOICE_INACTIVE_CLASS_NAME =
  "border-(--divider-subtle-color) bg-transparent text-(--text-muted) [&:not(:disabled):not(:has(input:disabled)):hover]:border-(--surface-interactive-hover-border) [&:not(:disabled):not(:has(input:disabled)):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):not(:has(input:disabled)):hover]:text-(--text-strong)";

const PICKER_CHOICE_BASE_CLASS_NAME =
  "flex h-10 radius-control-md px-3 text-md leading-6 tabular-nums";

const NUMERIC_CHOICE_ACTIVE_CLASS_NAME =
  "border-(--button-primary-border) bg-(--button-primary-background) text-(--button-primary-color)";

const NUMERIC_CHOICE_INACTIVE_CLASS_NAME =
  "border-transparent bg-transparent text-(--text-default) [&:not(:disabled):not(:has(input:disabled)):hover]:bg-(--surface-interactive-hover-background)";

const CALENDAR_CHOICE_BASE_CLASS_NAME =
  "flex h-8 radius-control-md ui-type-supporting tabular-nums";

const ICON_CHOICE_BASE_CLASS_NAME =
  "relative overflow-hidden p-0";

const ICON_CHOICE_SIZE_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "h-7 w-7 radius-control-sm",
  sm: "h-8 w-8 radius-control-md",
  md: "h-10 w-10 radius-control-lg",
  lg: "h-12 w-12 surface-radius-md",
};

const ICON_CHOICE_ACTIVE_CLASS_NAME =
  "border-(--surface-interactive-active-border) bg-(--surface-interactive-active-background)";

const ICON_CHOICE_INACTIVE_CLASS_NAME =
  "border-(--surface-inset-border) bg-transparent [&:not(:disabled):not(:has(input:disabled)):hover]:border-(--surface-interactive-hover-border) [&:not(:disabled):not(:has(input:disabled)):hover]:bg-(--surface-interactive-hover-background)";

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
  { active = false, muted = false, shape = "rounded", size = "md", tone = "primary", variant = "surface" }: UiChoiceStyleOptions,
  className?: string,
): string {
  return cn(CHOICE_BASE_CLASS_NAME, ...CHOICE_VARIANT_CLASS_RESOLVER[variant]({ active, muted, shape, size, tone, variant }), className);
}

function resolveSurfaceChoiceClasses({
  active,
  shape,
  size,
  tone,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    SURFACE_CHOICE_SIZE_CLASS_MAP[size],
    shape === "pill" ? "rounded-full" : SURFACE_CHOICE_ROUNDED_CLASS_MAP[size],
    active ? CHOICE_ACTIVE_CLASS_MAP[tone] : CHOICE_INACTIVE_CLASS_NAME,
  ];
}

function resolvePickerChoiceClasses({
  active,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    PICKER_CHOICE_BASE_CLASS_NAME,
    active ? NUMERIC_CHOICE_ACTIVE_CLASS_NAME : NUMERIC_CHOICE_INACTIVE_CLASS_NAME,
  ];
}

function resolveCalendarChoiceClasses({
  active,
  muted,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    CALENDAR_CHOICE_BASE_CLASS_NAME,
    active ? NUMERIC_CHOICE_ACTIVE_CLASS_NAME : NUMERIC_CHOICE_INACTIVE_CLASS_NAME,
    muted && !active && "text-(--text-muted)",
  ];
}

function resolveIconChoiceClasses({
  active,
  size,
}: ResolvedUiChoiceStyleOptions): ChoiceClassList {
  return [
    ICON_CHOICE_BASE_CLASS_NAME,
    ICON_CHOICE_SIZE_CLASS_MAP[size],
    active ? ICON_CHOICE_ACTIVE_CLASS_NAME : ICON_CHOICE_INACTIVE_CLASS_NAME,
  ];
}
