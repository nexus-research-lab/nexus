import { cn } from "@/lib/utils";

export type UiBadgeSize = "xs" | "sm" | "md";
export type UiBadgeTone =
  | "default"
  | "primary"
  | "success"
  | "warning"
  | "danger"
  | "info"
  | "idle"
  | "active"
  | "running";

interface UiBadgeStyleOptions {
  size?: UiBadgeSize;
  tone?: UiBadgeTone;
}

const BADGE_BASE_CLASS_NAME =
  "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-full border font-medium leading-none transition-[background,border-color,color] duration-(--motion-duration-fast)";

const BADGE_SIZE_CLASS_MAP: Record<UiBadgeSize, string> = {
  xs: "min-h-5 px-2 text-[10px]",
  sm: "min-h-6 px-2.5 text-[11px]",
  md: "min-h-7 px-3 text-[12px]",
};

const BADGE_TONE_CLASS_MAP: Record<UiBadgeTone, string> = {
  default:
    "border-(--chip-default-border) bg-(--chip-default-background) text-(--text-muted)",
  primary:
    "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] text-(--primary)",
  success:
    "border-[color:color-mix(in_srgb,var(--success)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-[color:color-mix(in_srgb,var(--success)_86%,var(--foreground)_14%)]",
  warning:
    "border-[color:color-mix(in_srgb,var(--warning)_28%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_12%,transparent)] text-[color:color-mix(in_srgb,var(--warning)_86%,var(--foreground)_14%)]",
  danger:
    "border-[color:color-mix(in_srgb,var(--destructive)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] text-(--destructive)",
  info:
    "border-[color:color-mix(in_srgb,var(--primary)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_8%,transparent)] text-[color:color-mix(in_srgb,var(--primary)_78%,var(--foreground)_22%)]",
  idle:
    "border-(--chip-default-border) bg-(--chip-default-background) text-(--text-soft)",
  active:
    "border-[color:color-mix(in_srgb,var(--primary)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_11%,transparent)] text-(--primary)",
  running:
    "border-[color:color-mix(in_srgb,var(--success)_24%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_12%,transparent)] text-[color:color-mix(in_srgb,var(--success)_86%,var(--foreground)_14%)]",
};

export function get_ui_badge_class_name(
  options: UiBadgeStyleOptions = {},
  class_name?: string,
): string {
  const {
    size = "sm",
    tone = "default",
  } = options;

  return cn(
    BADGE_BASE_CLASS_NAME,
    BADGE_SIZE_CLASS_MAP[size],
    BADGE_TONE_CLASS_MAP[tone],
    class_name,
  );
}
