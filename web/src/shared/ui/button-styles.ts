import { cn } from "@/shared/ui/class-name";

export type UiButtonTone = "default" | "primary" | "danger";
export type UiButtonVariant = "surface" | "solid" | "ghost" | "text";
export type UiButtonSize = "xs" | "sm" | "md" | "lg";
export type UiIconButtonSize = "xs" | "sm" | "md" | "lg";

interface UiButtonStyleOptions {
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiIconButtonStyleOptions {
  size?: UiIconButtonSize;
  tone?: UiButtonTone;
  variant?: Exclude<UiButtonVariant, "text">;
}

const BUTTON_BASE_CLASS_NAME =
  "inline-flex items-center justify-center gap-1.5 border font-semibold transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]";

const BUTTON_SIZE_CLASS_MAP: Record<UiButtonSize, string> = {
  xs: "min-h-7 rounded-[9px] px-2 py-1 text-[11px]",
  sm: "min-h-8 rounded-[10px] px-2.5 py-1.5 text-[12px]",
  md: "min-h-9 rounded-[12px] px-3.5 py-2 text-[13px]",
  lg: "min-h-10 rounded-[14px] px-4 py-2.5 text-sm",
};

const BUTTON_VARIANT_TONE_CLASS_MAP: Record<UiButtonVariant, Record<UiButtonTone, string>> = {
  surface: {
    default:
      "border-(--modal-btn-secondary-border) bg-(--modal-btn-secondary-background) text-(--text-default) hover:border-(--modal-btn-secondary-hover-border) hover:bg-(--modal-btn-secondary-hover-background) hover:text-(--text-strong)",
    primary:
      "border-[color:color-mix(in_srgb,var(--primary)_24%,var(--modal-btn-secondary-border))] bg-[color:color-mix(in_srgb,var(--primary)_8%,var(--modal-btn-secondary-background))] text-(--primary) hover:border-[color:color-mix(in_srgb,var(--primary)_34%,var(--modal-btn-secondary-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--primary)_12%,var(--modal-btn-secondary-hover-background))]",
    danger:
      "border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--modal-btn-secondary-border))] bg-(--modal-btn-secondary-background) text-(--destructive) hover:border-[color:color-mix(in_srgb,var(--destructive)_28%,var(--modal-btn-secondary-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_9%,var(--modal-btn-secondary-hover-background))]",
  },
  solid: {
    default:
      "border-(--button-tonal-border) bg-(--button-tonal-background) text-(--button-tonal-color) hover:bg-(--button-tonal-hover-background) hover:text-(--button-tonal-hover-color)",
    primary:
      "border-(--button-primary-border) bg-(--button-primary-background) text-(--button-primary-color) hover:border-(--button-primary-hover-border) hover:bg-(--button-primary-hover-background)",
    danger:
      "border-[color:color-mix(in_srgb,var(--destructive)_62%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_82%,white_18%)] text-white hover:border-[color:color-mix(in_srgb,var(--destructive)_74%,transparent)] hover:bg-[color:color-mix(in_srgb,var(--destructive)_88%,white_12%)]",
  },
  ghost: {
    default:
      "border-transparent bg-transparent text-(--text-default) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    primary:
      "border-transparent bg-transparent text-(--primary) hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--primary)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) hover:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
  },
  text: {
    default:
      "border-transparent bg-transparent text-(--text-muted) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
    primary:
      "border-transparent bg-transparent text-(--primary) hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--primary)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) hover:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
  },
};

const ICON_BUTTON_BASE_CLASS_NAME =
  "inline-flex items-center justify-center border transition-[background,border-color,color,box-shadow] duration-(--motion-duration-fast) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]";

const ICON_BUTTON_SIZE_CLASS_MAP: Record<UiIconButtonSize, string> = {
  xs: "h-6 w-6 rounded-[8px]",
  sm: "h-7 w-7 rounded-[9px]",
  md: "h-8 w-8 rounded-[10px]",
  lg: "h-9 w-9 rounded-[14px]",
};

const ICON_BUTTON_VARIANT_TONE_CLASS_MAP: Record<Exclude<UiButtonVariant, "text">, Record<UiButtonTone, string>> = {
  surface: BUTTON_VARIANT_TONE_CLASS_MAP.surface,
  solid: BUTTON_VARIANT_TONE_CLASS_MAP.solid,
  ghost: {
    default:
      "border-transparent bg-transparent text-(--icon-default) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-strong)",
    primary:
      "border-transparent bg-transparent text-(--primary) hover:border-[color:color-mix(in_srgb,var(--primary)_24%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--primary)_8%,var(--surface-interactive-hover-background))]",
    danger:
      "border-transparent bg-transparent text-(--destructive) hover:border-[color:color-mix(in_srgb,var(--destructive)_22%,var(--surface-interactive-hover-border))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,var(--surface-interactive-hover-background))]",
  },
};

/** 中文注释：按钮样式入口只在这里定义，业务组件通过 tone/variant/size 组合语义。 */
export function get_ui_button_class_name(
  options: UiButtonStyleOptions = {},
  class_name?: string,
): string {
  const {
    size = "md",
    tone = "default",
    variant = "surface",
  } = options;

  return cn(
    BUTTON_BASE_CLASS_NAME,
    BUTTON_SIZE_CLASS_MAP[size],
    BUTTON_VARIANT_TONE_CLASS_MAP[variant][tone],
    class_name,
  );
}

export function get_ui_icon_button_class_name(
  options: UiIconButtonStyleOptions = {},
  class_name?: string,
): string {
  const {
    size = "md",
    tone = "default",
    variant = "ghost",
  } = options;

  return cn(
    ICON_BUTTON_BASE_CLASS_NAME,
    ICON_BUTTON_SIZE_CLASS_MAP[size],
    ICON_BUTTON_VARIANT_TONE_CLASS_MAP[variant][tone],
    class_name,
  );
}
