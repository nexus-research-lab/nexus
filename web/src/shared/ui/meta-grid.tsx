"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";

interface UiMetaGridProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  class_name?: string;
}

interface UiMetaItemProps extends HTMLAttributes<HTMLDivElement> {
  class_name?: string;
  label: ReactNode;
  value: ReactNode;
}

export function UiMetaGrid({
  children,
  class_name,
  className,
  ...props
}: UiMetaGridProps) {
  return (
    <div
      className={cn("mt-3 grid gap-4 text-sm text-(--text-default) md:grid-cols-2", className, class_name)}
      {...props}
    >
      {children}
    </div>
  );
}

export function UiMetaItem({
  class_name,
  className,
  label,
  value,
  ...props
}: UiMetaItemProps) {
  return (
    <div className={cn("min-w-0", className, class_name)} {...props}>
      <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-(--text-muted)">
        {label}
      </p>
      <p className="mt-1.5 font-medium text-(--text-strong)">
        {value}
      </p>
    </div>
  );
}
