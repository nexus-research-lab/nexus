// INPUT: Button 的 size/tone/variant、原生 disabled/aria-busy、IconButton 的 shape 与调用方外部布局 class。
// OUTPUT: 由共享 token/recipe 组成的稳定按钮样式投影。
// POS: Button 视觉状态真相；不渲染 DOM，也不接受业务专属视觉覆盖。

import { cn } from "@/shared/ui/class-name";

export type UiButtonShape = "rounded" | "pill";
export type UiButtonTone = "default" | "primary" | "danger" | "success";
export type UiButtonVariant = "surface" | "outline" | "solid" | "ghost" | "text";
export type UiButtonSize = "2xs" | "xs" | "sm" | "md" | "lg";
export type UiIconButtonSize = "2xs" | "xs" | "sm" | "md" | "lg";
export type UiIconButtonShape = "rounded" | "round";

interface UiButtonStyleOptions {
  busy?: boolean;
  disabled?: boolean;
  shape?: UiButtonShape;
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiIconButtonStyleOptions {
  busy?: boolean;
  disabled?: boolean;
  shape?: UiIconButtonShape;
  size?: UiIconButtonSize;
  tone?: UiButtonTone;
  variant?: Exclude<UiButtonVariant, "text">;
}

const NEUTRAL_ACTIVE_BACKGROUND_CLASS_NAME =
  "active:bg-(--surface-interactive-active-background) aria-[checked=true]:bg-(--surface-interactive-active-background) aria-[current=page]:bg-(--surface-interactive-active-background) aria-[expanded=true]:bg-(--surface-interactive-active-background) aria-[pressed=true]:bg-(--surface-interactive-active-background)";
const NEUTRAL_ACTIVE_TEXT_CLASS_NAME =
  "aria-[checked=true]:text-(--text-strong) aria-[current=page]:text-(--text-strong) aria-[expanded=true]:text-(--text-strong) aria-[pressed=true]:text-(--text-strong)";
const NEUTRAL_ACTIVE_ICON_CLASS_NAME =
  "aria-[checked=true]:text-(--icon-strong) aria-[current=page]:text-(--icon-strong) aria-[expanded=true]:text-(--icon-strong) aria-[pressed=true]:text-(--icon-strong)";

const BUTTON_BASE_CLASS_NAME =
  "inline-flex items-center justify-center gap-2 border ui-type-weight-medium transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]";

const BUTTON_SIZE_CLASS_MAP: Record<UiButtonSize, string> = {
  "2xs": "min-h-6 px-1.5 py-0.5 ui-type-caption",
  xs: "min-h-7 px-2 py-1 ui-type-caption",
  sm: "min-h-8 px-2.5 py-1.5 ui-type-metadata",
  md: "min-h-9 px-3.5 py-1.5 ui-type-control",
  lg: "min-h-10 px-4 py-2 ui-type-control",
};

const BUTTON_ROUNDED_CLASS_MAP: Record<UiButtonSize, string> = {
  "2xs": "radius-control-xs",
  xs: "radius-control-xs",
  sm: "radius-control-sm",
  md: "radius-control-md",
  lg: "radius-control-lg",
};

const BUTTON_VARIANT_TONE_CLASS_MAP: Record<UiButtonVariant, Record<UiButtonTone, string>> = {
  surface: {
    default:
      "border-(--modal-btn-secondary-border) bg-(--modal-btn-secondary-background) text-(--text-default) [&:not(:disabled):hover]:border-(--modal-btn-secondary-hover-border) [&:not(:disabled):hover]:bg-(--modal-btn-secondary-hover-background) [&:not(:disabled):hover]:text-(--text-strong)",
    primary:
      "border-[color:color-mix(in_srgb,var(--brand-action)_24%,var(--modal-btn-secondary-border))] bg-[color:color-mix(in_srgb,var(--brand)_8%,var(--modal-btn-secondary-background))] text-(--brand-action) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--brand-action)_34%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--brand)_12%,var(--modal-btn-secondary-hover-background))]",
    danger:
      "border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--modal-btn-secondary-border))] bg-(--modal-btn-secondary-background) text-(--destructive) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_28%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_9%,var(--modal-btn-secondary-hover-background))]",
    success:
      "border-[color:color-mix(in_srgb,var(--success)_22%,var(--modal-btn-secondary-border))] bg-[color:color-mix(in_srgb,var(--success)_8%,var(--modal-btn-secondary-background))] text-(--success) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_32%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_12%,var(--modal-btn-secondary-hover-background))]",
  },
  outline: {
    default:
      "border-(--modal-btn-secondary-border) bg-transparent text-(--text-default) [&:not(:disabled):hover]:border-(--modal-btn-secondary-hover-border) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):hover]:text-(--text-strong)",
    primary:
      "border-[color:color-mix(in_srgb,var(--brand-action)_24%,var(--modal-btn-secondary-border))] bg-transparent text-(--brand-action) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--brand-action)_34%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--brand)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--modal-btn-secondary-border))] bg-transparent text-(--destructive) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_28%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
    success:
      "border-[color:color-mix(in_srgb,var(--success)_22%,var(--modal-btn-secondary-border))] bg-transparent text-(--success) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_32%,var(--modal-btn-secondary-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_8%,var(--surface-interactive-hover-background))]",
  },
  solid: {
    default:
      "border-(--button-tonal-border) bg-(--button-tonal-background) text-(--button-tonal-color) [&:not(:disabled):hover]:bg-(--button-tonal-hover-background) [&:not(:disabled):hover]:text-(--button-tonal-hover-color)",
    primary:
      "border-(--button-primary-border) bg-(--button-primary-background) text-(--button-primary-color) [&:not(:disabled):hover]:border-(--button-primary-hover-border) [&:not(:disabled):hover]:bg-(--button-primary-hover-background)",
    danger:
      "border-[color:color-mix(in_srgb,var(--destructive)_62%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_82%,white_18%)] text-white [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_74%,transparent)] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_88%,white_12%)]",
    success:
      "border-[color:color-mix(in_srgb,var(--success)_62%,transparent)] bg-(--success) text-white [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_74%,transparent)] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_88%,black_12%)]",
  },
  ghost: {
    default: cn(
      "border-transparent bg-transparent text-(--text-default) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):hover]:text-(--text-strong)",
      NEUTRAL_ACTIVE_BACKGROUND_CLASS_NAME,
      NEUTRAL_ACTIVE_TEXT_CLASS_NAME,
    ),
    primary:
      "border-transparent bg-transparent text-(--brand-action) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--brand-action)_24%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--brand)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
    success:
      "border-transparent bg-transparent text-(--success) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_8%,var(--surface-interactive-hover-background))]",
  },
  text: {
    default: cn(
      "border-transparent bg-transparent text-(--text-muted) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):hover]:text-(--text-strong)",
      NEUTRAL_ACTIVE_BACKGROUND_CLASS_NAME,
      NEUTRAL_ACTIVE_TEXT_CLASS_NAME,
    ),
    primary:
      "border-transparent bg-transparent text-(--brand-action) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--brand-action)_24%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--brand)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
    success:
      "border-transparent bg-transparent text-(--success) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_8%,var(--surface-interactive-hover-background))]",
  },
};

const ICON_BUTTON_BASE_CLASS_NAME =
  "inline-flex items-center justify-center border transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)]";

const ICON_BUTTON_SIZE_CLASS_MAP: Record<UiIconButtonSize, string> = {
  "2xs": "h-5 w-5",
  xs: "h-6 w-6",
  sm: "h-7 w-7",
  md: "h-8 w-8",
  lg: "h-9 w-9",
};

const ICON_BUTTON_ROUNDED_CLASS_MAP: Record<UiIconButtonSize, string> = {
  "2xs": "radius-control-xs",
  xs: "radius-control-xs",
  sm: "radius-control-sm",
  md: "radius-control-md",
  lg: "radius-control-lg",
};

const ICON_BUTTON_VARIANT_TONE_CLASS_MAP: Record<Exclude<UiButtonVariant, "text">, Record<UiButtonTone, string>> = {
  surface: BUTTON_VARIANT_TONE_CLASS_MAP.surface,
  outline: BUTTON_VARIANT_TONE_CLASS_MAP.outline,
  solid: BUTTON_VARIANT_TONE_CLASS_MAP.solid,
  ghost: {
    default: cn(
      "border-transparent bg-transparent text-(--icon-default) [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background) [&:not(:disabled):hover]:text-(--icon-strong)",
      NEUTRAL_ACTIVE_BACKGROUND_CLASS_NAME,
      NEUTRAL_ACTIVE_ICON_CLASS_NAME,
    ),
    primary:
      "border-transparent bg-transparent text-(--brand-action) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--brand-action)_24%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--brand)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
    success:
      "border-transparent bg-transparent text-(--success) [&:not(:disabled):hover]:border-[color:color-mix(in_srgb,var(--success)_22%,var(--surface-interactive-hover-border))] [&:not(:disabled):hover]:bg-[color:color-mix(in_srgb,var(--success)_8%,var(--surface-interactive-hover-background))]",
  },
};

function resolveButtonTone(tone: UiButtonTone, disabled: boolean, busy: boolean): UiButtonTone {
  // A disabled primary is unavailable unless the caller explicitly reports an
  // in-flight action. Never erase danger/success semantics or infer transactions.
  return disabled && !busy && tone === "primary" ? "default" : tone;
}

/** 中文注释：按钮样式入口只在这里定义，业务组件通过 tone/variant/size 组合语义。 */
export function getUiButtonClassName(
  options: UiButtonStyleOptions = {},
  className?: string,
): string {
  const {
    busy = false,
    disabled = false,
    shape = "rounded",
    size = "md",
    tone = "default",
    variant = "surface",
  } = options;

  return cn(
    BUTTON_BASE_CLASS_NAME,
    BUTTON_SIZE_CLASS_MAP[size],
    shape === "pill" ? "rounded-full" : BUTTON_ROUNDED_CLASS_MAP[size],
    BUTTON_VARIANT_TONE_CLASS_MAP[variant][resolveButtonTone(tone, disabled, busy)],
    className,
  );
}

export function getUiIconButtonClassName(
  options: UiIconButtonStyleOptions = {},
  className?: string,
): string {
  const {
    busy = false,
    disabled = false,
    shape = "rounded",
    size = "md",
    tone = "default",
    variant = "ghost",
  } = options;

  return cn(
    ICON_BUTTON_BASE_CLASS_NAME,
    ICON_BUTTON_SIZE_CLASS_MAP[size],
    shape === "round" ? "rounded-full" : ICON_BUTTON_ROUNDED_CLASS_MAP[size],
    ICON_BUTTON_VARIANT_TONE_CLASS_MAP[variant][resolveButtonTone(tone, disabled, busy)],
    className,
  );
}
