// INPUT: 加载占位的布局尺寸、语义明度和可选 DOM 属性。
// OUTPUT: 统一颜色、形状、动效与 reduced-motion 行为的装饰性骨架占位。
// POS: Display 层骨架屏视觉唯一所有者；业务消费者只负责排列和宽高。

"use client";

import { type HTMLAttributes } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiPanel } from "@/shared/ui/panel";

interface UiSkeletonProps extends HTMLAttributes<HTMLSpanElement> {
  className?: string;
  tone?: "default" | "strong" | "subtle";
}

interface UiSkeletonCardListProps {
  cardClassName?: string;
  className?: string;
  count?: number;
}

export function UiSkeleton({
  className,
  tone = "default",
  ...props
}: UiSkeletonProps) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "block rounded-full motion-safe:animate-pulse",
        SKELETON_TONE_CLASS_MAP[tone],
        className,
      )}
      {...props}
    />
  );
}

const SKELETON_TONE_CLASS_MAP = {
  default: "bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_62%,transparent)]",
  strong: "bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_76%,transparent)]",
  subtle: "bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_48%,transparent)]",
} as const;

export function UiSkeletonCardList({
  cardClassName: cardClassName,
  className: className,
  count = 3,
}: UiSkeletonCardListProps) {
  return (
    <div className={cn("space-y-3", className)}>
      {Array.from({ length: count }, (_, index) => (
        <UiPanel className={cn("min-h-[132px]", cardClassName)} key={index} padding="none" variant="dashed">
          <span className="sr-only">加载中</span>
        </UiPanel>
      ))}
    </div>
  );
}
