"use client";

import { type InputHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/lib/utils";

interface UiCheckboxRowProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "type"> {
  checked: boolean;
  class_name?: string;
  description?: ReactNode;
  label: ReactNode;
  on_change: (checked: boolean) => void;
}

export function UiCheckboxRow({
  checked,
  class_name,
  className,
  description,
  label,
  on_change,
  ...props
}: UiCheckboxRowProps) {
  return (
    <label
      className={cn(
        "flex items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-sm text-(--text-default)",
        "transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
        className,
        class_name,
      )}
    >
      <input
        checked={checked}
        className="h-4 w-4 shrink-0 accent-(--primary)"
        onChange={(event) => on_change(event.target.checked)}
        type="checkbox"
        {...props}
      />
      <span className="min-w-0">
        <span className="block font-medium text-(--text-default)">{label}</span>
        {description ? (
          <span className="mt-0.5 block text-xs leading-5 text-(--text-muted)">
            {description}
          </span>
        ) : null}
      </span>
    </label>
  );
}
