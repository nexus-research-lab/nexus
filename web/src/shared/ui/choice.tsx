"use client";

import { ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/lib/utils";
import {
  get_ui_choice_class_name,
  type UiChoiceShape,
  type UiChoiceSize,
  type UiChoiceTone,
  type UiChoiceVariant,
} from "@/shared/ui/choice-styles";

interface UiChoiceButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  active?: boolean;
  children: ReactNode;
  choice_size?: UiChoiceSize;
  class_name?: string;
  muted?: boolean;
  shape?: UiChoiceShape;
  tone?: UiChoiceTone;
  variant?: UiChoiceVariant;
}

export const UiChoiceButton = forwardRef<HTMLButtonElement, UiChoiceButtonProps>(function UiChoiceButton(
  {
    active = false,
    children,
    choice_size,
    class_name,
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
      className={get_ui_choice_class_name(
        { active, disabled, muted, shape, size: choice_size, tone, variant },
        cn(className, class_name),
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
