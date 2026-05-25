"use client";

import { type HTMLAttributes } from "react";

import { cn } from "@/lib/utils";
import { UiPanel } from "@/shared/ui/panel";

interface UiSkeletonProps extends HTMLAttributes<HTMLSpanElement> {
  class_name?: string;
}

interface UiSkeletonCardListProps {
  card_class_name?: string;
  class_name?: string;
  count?: number;
}

export function UiSkeleton({
  class_name,
  className,
  ...props
}: UiSkeletonProps) {
  return (
    <span
      className={cn(
        "block animate-pulse rounded-full bg-[color:color-mix(in_srgb,var(--surface-interactive-hover-background)_62%,transparent)]",
        className,
        class_name,
      )}
      {...props}
    />
  );
}

export function UiSkeletonCardList({
  card_class_name,
  class_name,
  count = 3,
}: UiSkeletonCardListProps) {
  return (
    <div className={cn("space-y-3", class_name)}>
      {Array.from({ length: count }, (_, index) => (
        <UiPanel class_name={cn("min-h-[132px]", card_class_name)} key={index} padding="none" variant="dashed">
          <span className="sr-only">加载中</span>
        </UiPanel>
      ))}
    </div>
  );
}
