import { cn } from "@/shared/ui/class-name";

export type UiFormControlSize = "xs" | "sm" | "md" | "lg";
export type UiFormControlVariant = "dialog" | "surface";

interface UiFormControlStyleOptions {
  multiline?: boolean;
  size?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

const FORM_CONTROL_BASE_CLASS_NAME =
  "w-full text-(--text-strong) outline-none transition-[background,border-color,box-shadow] duration-(--motion-duration-fast) placeholder:text-(--text-soft) focus-visible:outline-none aria-[invalid=true]:border-[color:color-mix(in_srgb,var(--destructive)_72%,transparent)] aria-[invalid=true]:shadow-[0_0_0_2px_color-mix(in_srgb,var(--destructive)_16%,transparent)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

const FORM_CONTROL_VARIANT_CLASS_MAP: Record<UiFormControlVariant, string> = {
  dialog: "dialog-input",
  surface: "input-shell",
};

const FORM_CONTROL_SIZE_CLASS_MAP: Record<UiFormControlSize, string> = {
  xs: "h-7 radius-control-xs px-2 text-compact",
  sm: "h-8 radius-control-sm px-3 text-sm",
  md: "h-9 radius-control-md px-3.5 text-sm",
  lg: "h-11 radius-control-lg px-4 text-base",
};

const FORM_TEXTAREA_SIZE_CLASS_MAP: Record<UiFormControlSize, string> = {
  xs: "min-h-16 radius-control-xs px-2 py-1.5 text-compact",
  sm: "min-h-20 radius-control-sm px-3 py-2 text-sm",
  md: "min-h-24 radius-control-md px-3.5 py-2.5 text-sm",
  lg: "min-h-28 radius-control-lg px-4 py-3 text-base",
};

const SEARCH_SHELL_SIZE_CLASS_MAP: Record<UiFormControlSize, string> = {
  xs: "h-7 radius-control-xs px-2 text-compact",
  sm: "h-8 radius-control-sm px-3 text-sm",
  md: "h-9 radius-control-md px-3.5 text-sm",
  lg: "h-11 radius-control-lg px-4 text-base",
};

const SEARCH_SHELL_VARIANT_CLASS_MAP: Record<UiFormControlVariant, string> = {
  dialog: "",
  surface: "ui-search-input-shell",
};

export function getUiFormControlClassName(
  options: UiFormControlStyleOptions = {},
  className?: string,
): string {
  const {
    multiline = false,
    size = "md",
    variant = "dialog",
  } = options;

  return cn(
    FORM_CONTROL_BASE_CLASS_NAME,
    FORM_CONTROL_VARIANT_CLASS_MAP[variant],
    multiline ? FORM_TEXTAREA_SIZE_CLASS_MAP[size] : FORM_CONTROL_SIZE_CLASS_MAP[size],
    className,
  );
}

export function getUiSearchInputShellClassName(
  options: Pick<UiFormControlStyleOptions, "size" | "variant"> = {},
  className?: string,
): string {
  const {
    size = "md",
    variant = "surface",
  } = options;

  return cn(
    "inline-flex min-w-0 items-center gap-2 text-(--text-default)",
    FORM_CONTROL_VARIANT_CLASS_MAP[variant],
    SEARCH_SHELL_VARIANT_CLASS_MAP[variant],
    SEARCH_SHELL_SIZE_CLASS_MAP[size],
    className,
  );
}
