"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

type UiPanelPadding = "none" | "sm" | "md" | "lg";
type UiPanelRadius = "sm" | "md" | "lg";
type UiPanelVariant = "card" | "inset" | "dashed" | "plain";

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
  sm: "rounded-[10px]",
  md: "rounded-[12px]",
  lg: "rounded-[14px]",
};

const PANEL_VARIANT_CLASS_MAP: Record<UiPanelVariant, string> = {
  card: "border border-(--divider-subtle-color) bg-transparent shadow-none",
  inset: "border border-(--divider-subtle-color) bg-transparent shadow-none",
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
