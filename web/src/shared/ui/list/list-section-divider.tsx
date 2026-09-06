// INPUT: 可选分组名称和标准 div 属性。
// OUTPUT: 由具名弱化标签与水平分隔线组成的列表分区边界，长名称可换行。
// POS: 跨目录列表的分组分隔 owner；不排序数据，也不解释分组业务含义。

"use client";

import { type HTMLAttributes, type ReactNode, useId } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface UiListSectionDividerProps extends HTMLAttributes<HTMLDivElement> {
  label?: ReactNode;
}

export function UiListSectionDivider({
  className,
  label,
  ...props
}: UiListSectionDividerProps) {
  const labelId = useId();
  const labelledBy = props["aria-labelledby"] ?? (
    label && props["aria-label"] === undefined ? labelId : undefined
  );
  return (
    <div
      aria-orientation="horizontal"
      className={cn("flex min-w-0 items-center gap-2 px-2.5 py-1.5", className)}
      role="separator"
      {...props}
      aria-labelledby={labelledBy}
    >
      {label ? (
        <span className={cn("min-w-0 [overflow-wrap:anywhere]", getUiTypographyClassName({
          role: "caption",
          tone: "soft",
          weight: "medium",
        }))} id={labelId}>
          {label}
        </span>
      ) : null}
      <span aria-hidden="true" className="h-px min-w-4 flex-1 bg-(--divider-subtle-color)" />
    </div>
  );
}
