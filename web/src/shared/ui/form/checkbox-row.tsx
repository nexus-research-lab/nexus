// INPUT: 带标签/说明的布尔值、可选图标、密度与变更命令。
// OUTPUT: 整行可点击且复用统一 checkbox、形状与排版角色的表单选择项。
// POS: Checkbox 行级 pattern；不拥有业务状态或提交事务。
"use client";

import { type InputHTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { UiCheckbox } from "@/shared/ui/form/checkbox";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface UiCheckboxRowProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "type"> {
  checked: boolean;
  className?: string;
  density?: "default" | "compact";
  description?: ReactNode;
  icon?: ReactNode;
  label: ReactNode;
  onChange: (checked: boolean) => void;
}

export function UiCheckboxRow({
  checked,
  className,
  density = "default",
  description,
  icon,
  label,
  onChange: onChange,
  ...props
}: UiCheckboxRowProps) {
  return (
    <label
      className={cn(
        "flex items-center border border-(--divider-subtle-color) bg-transparent text-(--text-default)",
        "transition-[background,border-color] duration-(--motion-duration-fast) hover:border-(--surface-interactive-hover-border) hover:bg-(--surface-interactive-hover-background)",
        density === "compact"
          ? "min-h-8 gap-1.5 radius-control-md px-2.5 py-1"
          : "gap-3 surface-radius-md px-4 py-3",
        className,
      )}
    >
      <UiCheckbox
        checked={checked}
        checkboxSize={density === "compact" ? "small" : "default"}
        onChange={(event) => onChange(event.target.checked)}
        {...props}
      />
      {icon}
      <span className="min-w-0">
        <span className={cn(
          "block",
          getUiTypographyClassName({
            role: density === "compact" ? "caption" : "control",
            tone: "default",
            weight: "medium",
          }),
        )}>
          {label}
        </span>
        {description ? (
          <span className={cn(
            "mt-0.5 block",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          )}>
            {description}
          </span>
        ) : null}
      </span>
    </label>
  );
}
