// INPUT: 带标签/说明的布尔值、装饰图标、密度、原生禁用与变更命令。
// OUTPUT: 名称/说明分离、整行原生命中且禁用时无 hover 的可换行复选项。
// POS: Checkbox 行级 pattern；拥有实例 ARIA 和密度，不拥有业务状态或提交事务。
"use client";

import { useId, type InputHTMLAttributes, type ReactNode } from "react";

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
  "aria-describedby": describedBy,
  "aria-label": ariaLabel,
  "aria-labelledby": labelledBy,
  checked,
  className,
  density = "default",
  description,
  icon,
  label,
  onChange,
  ...props
}: UiCheckboxRowProps) {
  const instanceId = useId();
  const labelId = `${instanceId}-label`;
  const descriptionId = `${instanceId}-description`;
  return (
    <label
      className={cn(
        "group/checkbox-row flex min-w-0 cursor-pointer items-center border border-(--divider-subtle-color) bg-transparent text-(--text-default)",
        "transition-[background,border-color] duration-(--motion-duration-fast) has-[:disabled]:cursor-not-allowed has-[:enabled]:hover:border-(--surface-interactive-hover-border) has-[:enabled]:hover:bg-(--surface-interactive-hover-background)",
        density === "compact"
          ? "min-h-8 gap-2 radius-control-md px-2.5 py-1"
          : "gap-3 surface-radius-md px-4 py-3",
        className,
      )}
      data-slot="checkbox-row"
    >
      <UiCheckbox
        aria-describedby={[describedBy, description ? descriptionId : undefined].filter(Boolean).join(" ") || undefined}
        aria-label={ariaLabel}
        aria-labelledby={labelledBy ?? (ariaLabel ? undefined : labelId)}
        checked={checked}
        checkboxSize={density === "compact" ? "small" : "default"}
        onChange={(event) => onChange(event.target.checked)}
        {...props}
      />
      {icon ? <span aria-hidden className="shrink-0 group-has-[:disabled]/checkbox-row:opacity-(--disabled-opacity)">{icon}</span> : null}
      <span className="min-w-0 break-words group-has-[:disabled]/checkbox-row:opacity-(--disabled-opacity)">
        <span id={labelId} className={cn(
          "block",
          getUiTypographyClassName({
            role: density === "compact" ? "supporting" : "control",
            tone: "default",
            weight: "medium",
          }),
        )}>
          {label}
        </span>
        {description ? (
          <span id={descriptionId} className={cn(
            "mt-0.5 block",
            getUiTypographyClassName({ role: "supporting", tone: "muted" }),
          )}>
            {description}
          </span>
        ) : null}
      </span>
    </label>
  );
}
