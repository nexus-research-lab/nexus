"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

interface UiListRowProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  active?: boolean;
  children?: ReactNode;
  className?: string;
  description?: ReactNode;
  leading?: ReactNode;
  meta?: ReactNode;
  onClick?: () => void;
  right?: ReactNode;
  subtitleTrailing?: ReactNode;
  title?: ReactNode;
}

export function UiListRow({
  actions,
  active = false,
  children,
  className,
  description,
  leading,
  meta,
  onClick: onClick,
  right,
  subtitleTrailing: subtitleTrailing,
  title,
  ...props
}: UiListRowProps) {
  return (
    <div
      className={cn(
        "group/item relative flex min-h-[68px] w-full items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-[background,color,transform] duration-(--motion-duration-fast)",
        onClick && "cursor-pointer",
        active
          ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-elevated-background))] text-(--text-strong) shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_12%,transparent)]"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
        className,
      )}
      {...props}
      onClick={onClick}
      onKeyDown={(event) => {
        props.onKeyDown?.(event);
        if (!onClick || event.defaultPrevented) {
          return;
        }
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onClick();
        }
      }}
      role={onClick ? "button" : undefined}
      tabIndex={onClick ? 0 : undefined}
    >
      {active ? (
        <span className="absolute left-0 top-1/2 h-9 w-[3px] -translate-y-1/2 rounded-full bg-(--primary)" />
      ) : null}

      {leading}

      {children ?? (
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-2">
            <span className="min-w-0 flex-1 truncate text-[14px] font-semibold">{title}</span>
            {meta}
          </div>
          {(description || subtitleTrailing) ? (
            <div className="mt-1 flex min-w-0 items-center gap-2">
              {description ? (
                <div className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-muted)">
                  {description}
                </div>
              ) : (
                <span className="min-w-0 flex-1" />
              )}
              {subtitleTrailing}
            </div>
          ) : null}
        </div>
      )}

      {right}
      {actions}
    </div>
  );
}
