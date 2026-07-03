"use client";

import { AnchorHTMLAttributes, ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  getUiButtonClassName,
  getUiIconButtonClassName,
  type UiButtonSize,
  type UiButtonTone,
  type UiButtonVariant,
  type UiIconButtonSize,
} from "@/shared/ui/button-styles";

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
      className={getUiIconButtonClassName({ size, tone, variant }, cn(className))}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
