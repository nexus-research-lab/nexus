// INPUT: 内容、有限的 padding/radius/variant 语义与原生 section 属性。
// OUTPUT: 默认无阴影的内容分组表面；透明 card、低透填充 filled、dashed 与 plain 各自表达真实内容边界。
// POS: 无交互 Panel primitive；不承载业务状态、标题结构或页面布局。
"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

type UiPanelPadding = "none" | "sm" | "md" | "lg";
type UiPanelRadius = "sm" | "md" | "lg";
type UiPanelVariant = "card" | "filled" | "dashed" | "plain";

interface UiPanelProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  className?: string;
  padding?: UiPanelPadding;
  radius?: UiPanelRadius;
  variant?: UiPanelVariant;
}

const PANEL_PADDING_CLASS_MAP: Record<UiPanelPadding, string> = {
  none: "",
  sm: "px-3 py-3",
  md: "px-4 py-4",
  lg: "px-5 py-5",
};

const PANEL_RADIUS_CLASS_MAP: Record<UiPanelRadius, string> = {
  sm: "surface-radius-sm",
  md: "surface-radius-md",
  lg: "surface-radius-lg",
};

const PANEL_VARIANT_CLASS_MAP: Record<UiPanelVariant, string> = {
  card: "border border-(--divider-subtle-color) bg-transparent shadow-none",
  filled: "border border-(--divider-subtle-color) bg-(--surface-panel-background) shadow-none",
  dashed: "border border-dashed border-(--divider-subtle-color) bg-transparent",
  plain: "",
};

export function UiPanel({
  children,
  className,
  padding = "md",
  radius = "md",
  variant = "card",
  ...props
}: UiPanelProps) {
  return (
    <section
      className={cn(
        PANEL_VARIANT_CLASS_MAP[variant],
        PANEL_RADIUS_CLASS_MAP[radius],
        PANEL_PADDING_CLASS_MAP[padding],
        className,
      )}
      {...props}
    >
      {children}
    </section>
  );
}
