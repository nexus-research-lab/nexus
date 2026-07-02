"use client";

import { ButtonHTMLAttributes, forwardRef, MouseEvent, ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  get_ui_list_action_class_name,
  type UiListActionShape,
  type UiListActionSize,
  type UiListActionTone,
  type UiListActionVisibility,
} from "@/shared/ui/list-action-styles";

interface UiListActionButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  class_name?: string;
  shape?: UiListActionShape;
  size?: UiListActionSize;
  stop_propagation?: boolean;
  tone?: UiListActionTone;
  visibility?: UiListActionVisibility;
}

export const UiListActionButton = forwardRef<HTMLButtonElement, UiListActionButtonProps>(function UiListActionButton(
  {
    children,
    class_name: legacyClassName,
    className,
    onClick,
    shape,
    size,
    stop_propagation: stopPropagation = false,
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
      className={get_ui_list_action_class_name(
        { shape, size, tone, visibility },
        cn(className, legacyClassName),
      )}
      onClick={handleClick}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
