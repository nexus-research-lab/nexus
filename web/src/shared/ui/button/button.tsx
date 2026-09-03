// INPUT: 用户动作、原生 button/link 属性与有限的 size/tone/variant 语义。
// OUTPUT: 默认安全为 type=button、可聚焦且具统一状态样式的文字/图标动作控件。
// POS: Button DOM 与可访问性原语；不判断业务权限、事务状态或页面布局。
"use client";

import { AnchorHTMLAttributes, ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiButtonClassName,
  getUiIconButtonClassName,
  type UiButtonSize,
  type UiButtonTone,
  type UiButtonVariant,
  type UiIconButtonSize,
} from "@/shared/ui/button/button-styles";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";

export type {
  UiButtonSize,
  UiButtonTone,
  UiButtonVariant,
  UiIconButtonSize,
} from "@/shared/ui/button/button-styles";

interface UiButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  className?: string;
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiLinkButtonProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  children: ReactNode;
  className?: string;
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiIconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  className?: string;
  size?: UiIconButtonSize;
  tone?: UiButtonTone;
  tooltip?: string | null;
  tooltipShortcut?: string;
  variant?: Exclude<UiButtonVariant, "text">;
}

export const UiButton = forwardRef<HTMLButtonElement, UiButtonProps>(function UiButton(
  {
    children,
    className,
    size,
    tone,
    type = "button",
    variant,
    ...props
  },
  ref,
) {
  return (
    <button
      ref={ref}
      className={getUiButtonClassName({ size, tone, variant }, cn(className))}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});

export const UiLinkButton = forwardRef<HTMLAnchorElement, UiLinkButtonProps>(function UiLinkButton(
  {
    children,
    className,
    size,
    tone,
    variant,
    ...props
  },
  ref,
) {
  return (
    <a
      ref={ref}
      className={getUiButtonClassName({ size, tone, variant }, cn(className))}
      {...props}
    >
      {children}
    </a>
  );
});

export const UiIconButton = forwardRef<HTMLButtonElement, UiIconButtonProps>(function UiIconButton(
  {
    "aria-label": ariaLabel,
    children,
    className,
    size,
    tone,
    title,
    tooltip,
    tooltipShortcut,
    type = "button",
    variant,
    ...props
  },
  ref,
) {
  const tooltipLabel = tooltip
    ?? (typeof title === "string" ? title : null)
    ?? (typeof ariaLabel === "string" ? ariaLabel : null);
  const button = (
    <button
      ref={ref}
      aria-label={ariaLabel ?? tooltipLabel ?? undefined}
      className={getUiIconButtonClassName({ size, tone, variant }, cn(className))}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
  return tooltipLabel ? (
    <UiTooltip label={tooltipLabel} shortcut={tooltipShortcut}>
      {button}
    </UiTooltip>
  ) : button;
});
