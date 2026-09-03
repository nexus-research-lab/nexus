// INPUT: 能力品牌的可选单色 SVG、可访问名称、回退字形与标准尺寸。
// OUTPUT: 使用统一中性容器、边框、圆角和主题前景色的品牌身份图标。
// POS: Capability 域品牌图标的唯一视觉所有者；不解释 Connector 或 Channel 状态。

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type CapabilityBrandIconSize = "sm" | "md" | "lg";

interface CapabilityBrandIconProps {
  className?: string;
  fallback?: ReactNode;
  size?: CapabilityBrandIconSize;
  src?: string;
  title: string;
}

const FRAME_SIZE_CLASS_NAMES: Record<CapabilityBrandIconSize, string> = {
  sm: "h-5 w-5 radius-control-xs",
  md: "h-9 w-9 radius-control-sm",
  lg: "h-14 w-14 surface-radius-md",
};

const MARK_SIZE_CLASS_NAMES: Record<CapabilityBrandIconSize, string> = {
  sm: "h-3.5 w-3.5",
  md: "h-6 w-6",
  lg: "h-9 w-9",
};

const FALLBACK_TYPE_ROLE = {
  sm: "caption",
  md: "metadata",
  lg: "objectTitle",
} as const;

export function CapabilityBrandIcon({
  className,
  fallback,
  size = "md",
  src,
  title,
}: CapabilityBrandIconProps) {
  return (
    <span
      aria-label={title}
      className={cn(
        "flex shrink-0 items-center justify-center overflow-hidden border border-(--divider-subtle-color) bg-(--surface-panel-background) text-(--text-strong)",
        FRAME_SIZE_CLASS_NAMES[size],
        className,
      )}
    >
      {src ? (
        <span
          aria-hidden="true"
          className={MARK_SIZE_CLASS_NAMES[size]}
          style={{
            backgroundColor: "var(--text-strong)",
            maskImage: `url(${src})`,
            maskPosition: "center",
            maskRepeat: "no-repeat",
            maskSize: "contain",
            WebkitMaskImage: `url(${src})`,
            WebkitMaskPosition: "center",
            WebkitMaskRepeat: "no-repeat",
            WebkitMaskSize: "contain",
          }}
        />
      ) : (
        <span
          aria-hidden="true"
          className={cn(
            "leading-none tracking-normal",
            getUiTypographyClassName({
              role: FALLBACK_TYPE_ROLE[size],
              tone: "strong",
              weight: "semibold",
            }),
          )}
        >
          {fallback}
        </span>
      )}
    </span>
  );
}
