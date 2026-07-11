"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";
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
        <div className="chip-default flex h-14 w-14 items-center justify-center rounded-[20px]">
          {icon}
        </div>
      ) : null}
      {title ? (
        <h3
          className={cn(
            "mt-5 text-lg font-bold tracking-[-0.03em]",
            tone === "danger" ? "text-(--destructive)" : "text-(--text-strong)",
            !icon && "mt-0",
          )}
        >
          {title}
        </h3>
      ) : null}
      {description ? (
        <p className="mt-2 max-w-md text-sm leading-6 text-(--text-default)">
          {description}
        </p>
      ) : null}
      {children}
      {actions ? <div className="mt-4 flex flex-wrap items-center justify-center gap-3">{actions}</div> : null}
    </div>
  );
}
