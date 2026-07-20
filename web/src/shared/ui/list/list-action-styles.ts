import { cn } from "@/shared/ui/class-name";

export type UiListActionShape = "round" | "rounded";
export type UiListActionSize = "xs" | "sm" | "md";
export type UiListActionTone = "default" | "danger";
export type UiListActionVisibility = "subtle" | "visible";

interface UiListActionStyleOptions {
  shape?: UiListActionShape;
  size?: UiListActionSize;
  tone?: UiListActionTone;
  visibility?: UiListActionVisibility;
}

const LIST_ACTION_BASE_CLASS_NAME =
  "inline-flex shrink-0 items-center justify-center border border-transparent text-(--icon-muted) transition-[background,border-color,color,opacity] duration-(--motion-duration-fast) focus-visible:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:color-mix(in_srgb,var(--primary)_22%,transparent)] disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)";

const LIST_ACTION_SIZE_CLASS_MAP: Record<UiListActionSize, string> = {
  xs: "h-6 w-6",
  sm: "h-7 w-7",
  md: "h-8 w-8",
};

const LIST_ACTION_SHAPE_CLASS_MAP: Record<UiListActionShape, string> = {
  round: "rounded-full",
  rounded: "rounded-[10px]",
};

const LIST_ACTION_TONE_CLASS_MAP: Record<UiListActionTone, string> = {
  default:
    "hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--icon-default)",
  danger:
    "hover:border-[color:color-mix(in_srgb,var(--destructive)_18%,var(--divider-subtle-color))] hover:bg-[color:color-mix(in_srgb,var(--destructive)_8%,transparent)] hover:text-(--destructive)",
};

const LIST_ACTION_VISIBILITY_CLASS_MAP: Record<UiListActionVisibility, string> = {
  subtle: "opacity-60 hover:opacity-100",
  visible: "opacity-100",
};

export function getUiListActionClassName(
  options: UiListActionStyleOptions = {},
  className?: string,
): string {
  const {
    shape = "rounded",
    size = "sm",
    tone = "default",
    visibility = "subtle",
  } = options;

  return cn(
    LIST_ACTION_BASE_CLASS_NAME,
    LIST_ACTION_SIZE_CLASS_MAP[size],
    LIST_ACTION_SHAPE_CLASS_MAP[shape],
    LIST_ACTION_TONE_CLASS_MAP[tone],
    LIST_ACTION_VISIBILITY_CLASS_MAP[visibility],
    className,
  );
}
