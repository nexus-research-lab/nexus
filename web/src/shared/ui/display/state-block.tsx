"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiStateBlockClassName,
  type UiStateBlockSize,
  type UiStateBlockTone,
  type UiStateBlockVariant,
} from "@/shared/ui/display/state-block-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface UiStateBlockProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  className?: string;
  description?: ReactNode;
  icon?: ReactNode;
  size?: UiStateBlockSize;
  title?: ReactNode;
  tone?: UiStateBlockTone;
  variant?: UiStateBlockVariant;
}

export function UiStateBlock({
  actions,
  children,
  className,
  description,
  icon,
  size,
  title,
  tone = "default",
  variant,
  ...props
}: UiStateBlockProps) {
  return (
    <div
      className={getUiStateBlockClassName(
        { size, tone, variant },
        cn(className),
      )}
      {...props}
    >
      {icon ? (
        <div
          className={cn(
            "chip-default flex items-center justify-center",
            tone === "default"
              ? "h-14 w-14 surface-radius-md"
              : "h-9 w-9 radius-control-md",
          )}
        >
          {icon}
        </div>
      ) : null}
      {title ? (
        <h3
          className={cn(
            tone === "default"
              ? getUiTypographyClassName({ role: "objectTitle", tone: "strong" })
              : getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
            tone === "default" ? "mt-5" : "mt-3",
            !icon && "mt-0",
          )}
        >
          {title}
        </h3>
      ) : null}
      {description ? (
        <p
          className={cn(
            "max-w-md",
            tone === "default"
              ? cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "default" }))
              : cn("mt-1.5", getUiTypographyClassName({ role: "metadata", tone: "default" })),
          )}
        >
          {description}
        </p>
      ) : null}
      {children}
      {actions ? <div className="mt-4 flex flex-wrap items-center justify-center gap-3">{actions}</div> : null}
    </div>
  );
}
