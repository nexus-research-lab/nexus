// INPUT: 状态标题、说明、装饰图标、正文/动作槽及有限样式语义。
// OUTPUT: 共享状态块排版、图标与长文本约束；播报由语义调用层负责。
// POS: 纯状态布局 owner，不订阅资源或派发领域命令。

"use client";

import { type HTMLAttributes, type ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiStateBlockClassName,
  type UiStateBlockSize,
  type UiStateBlockTone,
  type UiStateBlockVariant,
} from "@/shared/ui/display/state-block-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
        <div
          aria-hidden="true"
          className={cn(
            "chip-default flex shrink-0 items-center justify-center",
            tone === "default"
              ? "h-14 w-14 surface-radius-md"
              : "h-9 w-9 radius-control-md",
          )}
        >
          {icon}
        </div>
      ) : null}
      {title ? (
        <h3
          className={cn(
            "max-w-full",
            tone === "default"
              ? getUiTypographyClassName({ role: "objectTitle", tone: "strong" })
              : getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
            tone === "default" ? "mt-5" : "mt-3",
            !icon && "mt-0",
          )}
        >
          {title}
        </h3>
      ) : null}
      {description ? (
        <p
          className={cn(
            "w-full max-w-md",
            tone === "default"
              ? cn("mt-2", getUiTypographyClassName({ role: "supporting", tone: "default" }))
              : cn("mt-1.5", getUiTypographyClassName({ role: "metadata", tone: "default" })),
          )}
        >
          {description}
        </p>
      ) : null}
      {children}
      {actions ? <div className="mt-4 flex max-w-full flex-wrap items-center justify-center gap-3">{actions}</div> : null}
    </div>
  );
}
