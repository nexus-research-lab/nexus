import { cn } from "@/shared/ui/class-name";

export type UiChoiceTone = "primary" | "danger" | "success";
export type UiChoiceVariant = "surface" | "picker" | "calendar";
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
  xs: "min-h-7 px-2 py-1 text-[11px]",
  sm: "min-h-8 px-2.5 py-1.5 text-[12px]",
  md: "min-h-9 px-3 py-2 text-[12px]",
  lg: "min-h-10 px-3.5 py-2.5 text-sm",
};

const SURFACE_CHOICE_ROUNDED_CLASS_MAP: Record<UiChoiceSize, string> = {
  xs: "rounded-[9px]",
  sm: "rounded-[10px]",
  md: "rounded-[12px]",
  lg: "rounded-[14px]",
};

const CHOICE_ACTIVE_CLASS_MAP: Record<UiChoiceTone, string> = {
  primary:
    "border-[color:color-mix(in_srgb,var(--primary)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-(--primary)",
  danger:
    "border-[color:color-mix(in_srgb,var(--destructive)_24%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--destructive)_9%,transparent)] text-(--destructive)",
  success:
    "border-[color:color-mix(in_srgb,var(--success)_28%,var(--divider-subtle-color))] bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] text-(--success)",
};

const CHOICE_INACTIVE_CLASS_NAME =
  "border-(--divider-subtle-color) bg-transparent text-(--text-muted) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)";

const PICKER_CHOICE_BASE_CLASS_NAME =
  "flex h-10 items-center justify-center rounded-[10px] border px-3 text-[17px] font-semibold transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-40";

const PICKER_CHOICE_ACTIVE_CLASS_NAME =
  "border-[color:color-mix(in_srgb,var(--primary)_58%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_88%,white)] text-white shadow-[0_8px_18px_color-mix(in_srgb,var(--primary)_22%,transparent)]";

const PICKER_CHOICE_INACTIVE_CLASS_NAME =
  "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background)";

const CALENDAR_CHOICE_BASE_CLASS_NAME =
  "flex h-8 items-center justify-center rounded-[10px] border text-xs font-semibold transition-[background,border-color,color] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-40";

const CALENDAR_CHOICE_ACTIVE_CLASS_NAME =
  "border-[color:color-mix(in_srgb,var(--primary)_58%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_90%,white)] text-white";

const CALENDAR_CHOICE_INACTIVE_CLASS_NAME =
  "border-transparent bg-transparent text-(--text-default) hover:bg-(--surface-interactive-hover-background)";

const CHOICE_VARIANT_CLASS_RESOLVER: Record<
  UiChoiceVariant,
  ChoiceVariantClassResolver
> = {
  calendar: resolveCalendarChoiceClasses,
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

function optionOrDefault<T>(value: T | undefined, fallback: T): T {
  return value === undefined ? fallback : value;
}
