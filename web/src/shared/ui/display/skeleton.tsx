"use client";

import { type HTMLAttributes } from "react";

import { cn } from "@/lib/utils";
import { UiPanel } from "@/shared/ui/panel";

interface UiSkeletonProps extends HTMLAttributes<HTMLSpanElement> {
  className?: string;
}

interface UiSkeletonCardListProps {
  cardClassName?: string;
  className?: string;
  count?: number;
}

export function UiSkeleton({
  className,
  ...props
}: UiSkeletonProps) {
  return (
    <span
      className={cn(
        "block animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_62%,transparent)]",
        className,
      )}
      {...props}
    />
  );
}

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
