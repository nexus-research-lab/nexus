// INPUT: 可选分组名称和标准 div 属性。
// OUTPUT: 由弱化标签与水平分隔线组成的列表分区边界。
// POS: 跨目录列表的分组分隔 owner；不排序数据，也不解释分组业务含义。

"use client";

import type { HTMLAttributes, ReactNode } from "react";

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
  return (
    <div
      aria-orientation="horizontal"
      className={cn("flex items-center gap-2 px-2.5 py-1.5", className)}
      role="separator"
      {...props}
    >
      {label ? (
        <span className={getUiTypographyClassName({
          role: "caption",
          tone: "soft",
          weight: "medium",
        })}>
          {label}
        </span>
      ) : null}
      <span aria-hidden="true" className="h-px min-w-4 flex-1 bg-(--divider-subtle-color)" />
    </div>
  );
}
