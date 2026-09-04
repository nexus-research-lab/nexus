// INPUT: 单个可切换选项的 active/checked/disabled 状态与 size/tone/variant 语义。
// OUTPUT: 统一的选择按钮，或保留原生 radio 语义的整项选择控件。
// POS: Choice 原语；不拥有选项集合、业务选择值或提交行为。
"use client";

import {
  ButtonHTMLAttributes,
  forwardRef,
  InputHTMLAttributes,
  ReactNode,
} from "react";

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
      data-disabled={disabled || undefined}
      disabled={disabled}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
});

interface UiRadioChoiceProps extends Omit<
  InputHTMLAttributes<HTMLInputElement>,
  "checked" | "className" | "size" | "type"
> {
  checked: boolean;
  children: ReactNode;
  choiceSize?: UiChoiceSize;
  className?: string;
  muted?: boolean;
  shape?: UiChoiceShape;
  tone?: UiChoiceTone;
  variant?: UiChoiceVariant;
}

export const UiRadioChoice = forwardRef<HTMLInputElement, UiRadioChoiceProps>(
  function UiRadioChoice(
    {
      checked,
      children,
      choiceSize,
      className,
      disabled,
      muted,
      shape,
      tone,
      variant,
      ...props
    },
    ref,
  ) {
    return (
      <label
        className={getUiChoiceClassName(
          {
            active: checked,
            disabled,
            muted,
            shape,
            size: choiceSize,
            tone,
            variant,
          },
          cn(
            "has-[input:focus-visible]:ring-2 has-[input:focus-visible]:ring-[color:color-mix(in_srgb,var(--primary)_24%,transparent)]",
            disabled && "cursor-not-allowed opacity-(--disabled-opacity)",
            className,
          ),
        )}
        data-active={checked}
        data-disabled={disabled || undefined}
      >
        <input
          {...props}
          ref={ref}
          checked={checked}
          className="sr-only"
          disabled={disabled}
          type="radio"
        />
        {children}
      </label>
    );
  },
);
