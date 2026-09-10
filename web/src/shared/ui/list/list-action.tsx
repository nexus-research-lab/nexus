// INPUT: 列表次动作、展示时机与原生 button 属性。
// OUTPUT: 复用 IconButton 尺寸、tone、禁用、焦点与 Tooltip 的独立列表动作。
// POS: ListAction 只拥有事件隔离与行内可见性；不重复实现按钮 DOM 或视觉状态。
"use client";

import { ButtonHTMLAttributes, forwardRef, MouseEvent, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  UiIconButton,
  type UiButtonTone,
  type UiIconButtonShape,
  type UiIconButtonSize,
} from "@/shared/ui/button/button";
import {
  getUiListActionVisibilityClassName,
  type UiListActionVisibility,
} from "@/shared/ui/list/list-action-styles";

export type UiListActionTone = Exclude<UiButtonTone, "success">;

interface UiListActionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  className?: string;
  shape?: UiIconButtonShape;
  size?: Extract<UiIconButtonSize, "xs" | "sm" | "md">;
  stopPropagation?: boolean;
  tone?: UiListActionTone;
  visibility?: UiListActionVisibility;
}

export const UiListActionButton = forwardRef<HTMLButtonElement, UiListActionButtonProps>(function UiListActionButton(
  {
    children,
    className,
    onClick,
    shape,
    size = "sm",
    stopPropagation: stopPropagation = false,
    tone,
    type = "button",
    visibility = "subtle",
    ...props
  },
  ref,
) {
  const handleClick = (event: MouseEvent<HTMLButtonElement>) => {
    if (stopPropagation) {
      event.stopPropagation();
    }
    onClick?.(event);
  };

  return (
    <UiIconButton
      ref={ref}
      className={cn(getUiListActionVisibilityClassName(visibility), className)}
      onClick={handleClick}
      shape={shape}
      size={size}
      tone={tone}
      type={type}
      {...props}
    >
      {children}
    </UiIconButton>
  );
});
