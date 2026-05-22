"use client";

import { LucideIcon } from "lucide-react";

import { UiSegmentedControl } from "@/shared/ui/segmented-control";

interface SegmentedPillOption<T extends string> {
  label: string;
  value: T;
}

interface SegmentedPillProps<T extends string> {
  class_name?: string;
  density?: "default" | "compact";
  icon?: LucideIcon;
  on_change: (value: T) => void;
  options: SegmentedPillOption<T>[];
  stretch?: boolean;
  title: string;
  value: T;
}

/** 兼容旧命名，新代码优先使用 UiSegmentedControl。 */
export function SegmentedPill<T extends string>(props: SegmentedPillProps<T>) {
  return <UiSegmentedControl {...props} />;
}
