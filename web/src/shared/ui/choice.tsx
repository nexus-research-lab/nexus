"use client";

import { ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  getUiChoiceClassName,
  type UiChoiceShape,
  type UiChoiceSize,
  type UiChoiceTone,
  type UiChoiceVariant,
} from "@/shared/ui/choice-styles";

interface UiChoiceButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  children: ReactNode;
  choiceSize?: UiChoiceSize;
  className?: string;
  muted?: boolean;
  shape?: UiChoiceShape;
  tone?: UiChoiceTone;
  variant?: UiChoiceVariant;
}

export const UiChoiceButton = forwardRef<HTMLButtonElement, UiChoiceButtonProps>(function UiChoiceButton(
  {
    active = false,
    children,
    choiceSize: choiceSize,
    className,
    disabled,
    muted,
    shape,
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
      aria-pressed={active}
      className={getUiChoiceClassName(
        { active, disabled, muted, shape, size: choiceSize, tone, variant },
        cn(className),
      )}
      data-active={active}
      disabled={disabled}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});
