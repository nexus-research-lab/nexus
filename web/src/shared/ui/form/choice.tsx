// INPUT: 单个可切换选项的 active/disabled 状态与 size/tone/variant 语义。
// OUTPUT: 以 aria-pressed 表达选择状态的统一选择按钮。
// POS: Choice button 原语；不拥有选项集合、业务选择值或提交行为。
"use client";

import { ButtonHTMLAttributes, forwardRef, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import {
  getUiChoiceClassName,
  type UiChoiceShape,
  type UiChoiceSize,
  type UiChoiceTone,
  type UiChoiceVariant,
} from "@/shared/ui/form/choice-styles";

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
