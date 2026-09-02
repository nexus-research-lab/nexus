// INPUT: 带标签/说明的布尔值与变更命令。
// OUTPUT: 整行可点击且复用统一 checkbox 原语的表单选择项。
// POS: Checkbox 行级 pattern；不拥有业务状态或提交事务。
"use client";

import { type InputHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiCheckbox } from "@/shared/ui/form/checkbox";

interface UiCheckboxRowProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "type"> {
  checked: boolean;
  className?: string;
  description?: ReactNode;
  label: ReactNode;
  onChange: (checked: boolean) => void;
}

export function UiCheckboxRow({
  checked,
  className,
  description,
  label,
  onChange: onChange,
  ...props
}: UiCheckboxRowProps) {
  return (
    <label
      className={cn(
        "flex items-center gap-3 rounded-[12px] border border-(--divider-subtle-color) bg-transparent px-4 py-3 text-sm text-(--text-default)",
        "transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
        className,
      )}
    >
      <UiCheckbox
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
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
