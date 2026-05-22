"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";

interface UiListRowProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  active?: boolean;
  children?: ReactNode;
  class_name?: string;
  description?: ReactNode;
  leading?: ReactNode;
  meta?: ReactNode;
  on_click?: () => void;
  right?: ReactNode;
  subtitle_trailing?: ReactNode;
  title?: ReactNode;
}

export function UiListRow({
  actions,
  active = false,
  children,
  class_name,
  className,
  description,
  leading,
  meta,
  on_click,
  right,
  subtitle_trailing,
  title,
  ...props
}: UiListRowProps) {
  return (
    <div
      className={cn(
        "group/item relative flex min-h-[68px] w-full items-center gap-3 rounded-[14px] px-3 py-2.5 text-left transition-[background,color,transform] duration-(--motion-duration-fast)",
        on_click && "cursor-pointer",
        active
          ? "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-elevated-background))] text-(--text-strong) shadow-[inset_0_0_0_1px_color-mix(in_srgb,var(--primary)_12%,transparent)]"
          : "text-(--text-default) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
        className,
        class_name,
      )}
      {...props}
      onClick={on_click}
      onKeyDown={(event) => {
        props.onKeyDown?.(event);
        if (!on_click || event.defaultPrevented) {
          return;
        }
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          on_click();
        }
      }}
      role={on_click ? "button" : undefined}
      tabIndex={on_click ? 0 : undefined}
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
          {(description || subtitle_trailing) ? (
            <div className="mt-1 flex min-w-0 items-center gap-2">
              {description ? (
                <span className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-muted)">
                  {description}
                </span>
              ) : (
                <span className="min-w-0 flex-1" />
              )}
              {subtitle_trailing}
            </div>
          ) : null}
        </div>
      )}

      {right}
      {actions}
    </div>
  );
}
