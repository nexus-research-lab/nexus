"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";

type UiPanelPadding = "none" | "sm" | "md" | "lg";
type UiPanelRadius = "sm" | "md" | "lg";
type UiPanelVariant = "card" | "inset" | "dashed" | "plain";

interface UiPanelProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  class_name?: string;
  padding?: UiPanelPadding;
  radius?: UiPanelRadius;
  variant?: UiPanelVariant;
}

interface UiSectionHeaderProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  action?: ReactNode;
  children?: ReactNode;
  class_name?: string;
  description?: ReactNode;
  icon?: ReactNode;
  title: ReactNode;
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
  class_name,
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
        class_name,
      )}
      {...props}
    >
      {children}
    </section>
  );
}

export function UiSectionHeader({
  action,
  children,
  class_name,
  className,
  description,
  icon,
  title,
  ...props
}: UiSectionHeaderProps) {
  return (
    <div className={cn("flex items-start justify-between gap-3", className, class_name)} {...props}>
      {children ?? (
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            {icon ? <span className="shrink-0 text-(--icon-default)">{icon}</span> : null}
            <div className="min-w-0">
              <h2 className="text-[15px] font-semibold tracking-[-0.03em] text-(--text-strong)">
                {title}
              </h2>
              {description ? (
                <p className="text-xs leading-5 text-(--text-default)">
                  {description}
                </p>
              ) : null}
            </div>
          </div>
        </div>
      )}
      {action}
    </div>
  );
}
