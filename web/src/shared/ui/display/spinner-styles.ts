// INPUT: 加载指示器的有限尺寸、语义颜色和可选外部布局 class。
// OUTPUT: 统一的旋转、减弱动效、尺寸与颜色 class；不创建 live region 或加载文案。
// POS: shared/ui 瞬时加载图标的视觉合同；可访问状态仍由真实业务容器负责。

import { cn } from "@/shared/ui/class-name";

export type UiSpinnerSize = "xs" | "sm" | "md" | "lg" | "xl";
export type UiSpinnerTone = "current" | "muted" | "primary";

interface UiSpinnerStyleOptions {
  size?: UiSpinnerSize;
  tone?: UiSpinnerTone;
}

const SPINNER_SIZE_CLASS_MAP: Record<UiSpinnerSize, string> = {
  xs: "h-3 w-3",
  sm: "h-3.5 w-3.5",
  md: "h-4 w-4",
  lg: "h-5 w-5",
  xl: "h-6 w-6",
};

const SPINNER_TONE_CLASS_MAP: Record<UiSpinnerTone, string> = {
  current: "text-current",
  muted: "text-(--icon-muted)",
  primary: "text-primary",
};

export function getUiSpinnerClassName(
  options: UiSpinnerStyleOptions = {},
  className?: string,
): string {
  const { size = "md", tone = "current" } = options;

  return cn(
    "shrink-0 animate-spin motion-reduce:animate-none",
    SPINNER_SIZE_CLASS_MAP[size],
    SPINNER_TONE_CLASS_MAP[tone],
    className,
  );
}
