"use client";

import { ButtonHTMLAttributes, forwardRef, MouseEvent, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiListActionClassName,
  type UiListActionShape,
  type UiListActionSize,
  type UiListActionTone,
  type UiListActionVisibility,
} from "@/shared/ui/list/list-action-styles";

interface UiListActionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  className?: string;
  shape?: UiListActionShape;
  size?: UiListActionSize;
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
    size,
    stopPropagation: stopPropagation = false,
    tone,
    type = "button",
    visibility,
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
    <button
      ref={ref}
      className={getUiListActionClassName(
        { shape, size, tone, visibility },
        cn(className),
      )}
      onClick={handleClick}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
