"use client";

import { ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/lib/utils";

type WorkspacePillButtonVariant = "primary" | "outlined" | "tonal" | "text" | "icon";
type WorkspacePillButtonTone = "default" | "danger";

interface WorkspacePillButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "className"> {
  children: ReactNode;
  variant?: WorkspacePillButtonVariant;
  tone?: WorkspacePillButtonTone;
  size?: "sm" | "md" | "lg" | "icon";
  density?: "default" | "compact";
  stretch?: boolean;
  /** 中文注释：这里只允许布局层补充外边距、显隐和定位，不再覆写颜色、圆角和阴影。 */
  class_name?: string;
}

export const WorkspacePillButton = forwardRef<HTMLButtonElement, WorkspacePillButtonProps>(
  function WorkspacePillButton({
    children,
    class_name,
    type = "button",
    variant,
    tone = "default",
    size = "md",
    density = "default",
    stretch = false,
    ...props
  }: WorkspacePillButtonProps, ref) {
    const resolved_variant = variant ?? (size === "icon" ? "icon" : "tonal");

    return (
      <button
        className={cn(
          "chip-button disabled:cursor-not-allowed disabled:opacity-60",
          class_name,
        )}
        data-density={density}
        data-size={size}
        data-stretch={stretch}
        data-tone={tone}
        data-variant={resolved_variant}
        ref={ref}
        type={type}
        {...props}
      >
        {children}
      </button>
    );
  },
);
