// INPUT: Badge 的 size、tone、shape 与调用方外部布局 class。
// OUTPUT: 由共享 token/recipe 组成的稳定 Badge 样式投影。
// POS: Badge 视觉状态真相；不渲染 DOM，也不接受业务专属形状覆盖。

import { cn } from "@/shared/ui/class-name";

export type UiBadgeSize = "xs" | "sm" | "md";
export type UiBadgeShape = "rounded" | "pill";
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
  shape?: UiBadgeShape;
  size?: UiBadgeSize;
  tone?: UiBadgeTone;
}

const BADGE_BASE_CLASS_NAME =
  "inline-flex shrink-0 items-center justify-center gap-1 border font-medium leading-none transition-[background,border-color,color] duration-(--motion-duration-fast)";

const BADGE_SHAPE_CLASS_MAP: Record<UiBadgeShape, string> = {
  rounded: "radius-control-xs",
  pill: "rounded-full",
};

const BADGE_SIZE_CLASS_MAP: Record<UiBadgeSize, string> = {
  xs: "min-h-5 px-1.5 text-2xs",
  sm: "min-h-[22px] px-2 text-xs",
  md: "min-h-6 px-2.5 text-compact",
};

const BADGE_TONE_CLASS_MAP: Record<UiBadgeTone, string> = {
  default:
    "border-(--divider-subtle-color) bg-transparent text-(--text-muted)",
  primary:
    "border-[color:color-mix(in_srgb,var(--primary)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_5%,transparent)] text-(--primary)",
  success:
    "border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_6%,transparent)] text-[color:color-mix(in_srgb,var(--success)_86%,var(--foreground)_14%)]",
  warning:
    "border-[color:color-mix(in_srgb,var(--warning)_20%,transparent)] bg-[color:color-mix(in_srgb,var(--warning)_6%,transparent)] text-[color:color-mix(in_srgb,var(--warning)_86%,var(--foreground)_14%)]",
  danger:
    "border-[color:color-mix(in_srgb,var(--destructive)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--destructive)_5%,transparent)] text-(--destructive)",
  info:
    "border-[color:color-mix(in_srgb,var(--primary)_16%,transparent)] bg-[color:color-mix(in_srgb,var(--primary)_4%,transparent)] text-[color:color-mix(in_srgb,var(--primary)_78%,var(--foreground)_22%)]",
  idle:
    "border-(--divider-subtle-color) bg-transparent text-(--text-soft)",
  active:
    "border-[color:color-mix(in_srgb,var(--success)_18%,transparent)] bg-[color:color-mix(in_srgb,var(--success)_6%,transparent)] text-[color:color-mix(in_srgb,var(--success)_86%,var(--foreground)_14%)]",
  running:
    "border-[color:var(--status-running-soft-border)] bg-[var(--status-running-soft-bg)] text-[var(--status-running-soft-text)]",
};

export function getUiBadgeClassName(
  options: UiBadgeStyleOptions = {},
  className?: string,
): string {
  const {
    shape = "rounded",
    size = "sm",
    tone = "default",
  } = options;

  return cn(
    BADGE_BASE_CLASS_NAME,
    BADGE_SHAPE_CLASS_MAP[shape],
    BADGE_SIZE_CLASS_MAP[size],
    BADGE_TONE_CLASS_MAP[tone],
    className,
  );
}
