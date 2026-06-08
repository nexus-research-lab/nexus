"use client";

import { AnchorHTMLAttributes, ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  get_ui_button_class_name,
  get_ui_icon_button_class_name,
  type UiButtonSize,
  type UiButtonTone,
  type UiButtonVariant,
  type UiIconButtonSize,
} from "@/shared/ui/button-styles";

interface UiButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  class_name?: string;
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiLinkButtonProps extends AnchorHTMLAttributes<HTMLAnchorElement> {
  children: ReactNode;
  class_name?: string;
  size?: UiButtonSize;
  tone?: UiButtonTone;
  variant?: UiButtonVariant;
}

interface UiIconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  class_name?: string;
  size?: UiIconButtonSize;
  tone?: UiButtonTone;
  variant?: Exclude<UiButtonVariant, "text">;
}

export const UiButton = forwardRef<HTMLButtonElement, UiButtonProps>(function UiButton(
  {
    children,
    class_name,
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
      className={get_ui_button_class_name({ size, tone, variant }, cn(className, class_name))}
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
    class_name,
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
      className={get_ui_button_class_name({ size, tone, variant }, cn(className, class_name))}
      {...props}
    >
      {children}
    </a>
  );
});

export const UiIconButton = forwardRef<HTMLButtonElement, UiIconButtonProps>(function UiIconButton(
  {
    children,
    class_name,
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
      className={get_ui_icon_button_class_name({ size, tone, variant }, cn(className, class_name))}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
