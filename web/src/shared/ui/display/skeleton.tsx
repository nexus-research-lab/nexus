// INPUT: 加载占位的布局尺寸、语义明度和可选 DOM 属性。
// OUTPUT: 装饰性骨架占位，以及仅播报一次本地化等待文案的卡片占位组。
// POS: Display 层骨架屏视觉唯一所有者；业务消费者只负责排列和宽高。

"use client";

import { type HTMLAttributes } from "react";

import { cn } from "@/shared/ui/class-name";
import { useI18n } from "@/shared/i18n/i18n-context";
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
  cardClassName,
  className,
  count = 3,
}: UiSkeletonCardListProps) {
  const { t } = useI18n();
  return (
    <div aria-busy="true" className={cn("space-y-3", className)} role="status">
      <span className="sr-only">{t("common.loading")}</span>
      {Array.from({ length: count }, (_, index) => (
        <UiPanel aria-hidden="true" className={cn("min-h-[132px]", cardClassName)} key={index} padding="none" variant="dashed">{null}</UiPanel>
      ))}
    </div>
  );
}
