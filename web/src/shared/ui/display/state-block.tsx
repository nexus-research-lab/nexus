"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiStateBlockClassName,
  type UiStateBlockSize,
  type UiStateBlockTone,
  type UiStateBlockVariant,
} from "@/shared/ui/display/state-block-styles";

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
              : "h-9 w-9 rounded-[9px]",
          )}
        >
          {icon}
        </div>
      ) : null}
      {title ? (
        <h3
          className={cn(
            tone === "default"
              ? "mt-5 text-lg font-semibold tracking-[-0.03em]"
              : "mt-3 text-sm font-semibold tracking-[-0.015em]",
            "text-(--text-strong)",
            !icon && "mt-0",
          )}
        >
          {title}
        </h3>
      ) : null}
      {description ? (
        <p
          className={cn(
            "max-w-md text-(--text-default)",
            tone === "default"
              ? "mt-2 text-sm leading-6"
              : "mt-1.5 text-xs leading-5",
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
