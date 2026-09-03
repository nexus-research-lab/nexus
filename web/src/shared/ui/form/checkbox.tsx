// INPUT: 原生 checkbox 属性、三态值与 small/default 两档控件尺寸。
// OUTPUT: 统一品牌色、焦点环、mixed ARIA 和 disabled 状态的原生 checkbox。
// POS: Checkbox DOM 原语；不渲染标签、说明或业务选择逻辑。
"use client";

import {
  forwardRef,
  type InputHTMLAttributes,
  useEffect,
  useImperativeHandle,
  useRef,
} from "react";

import { cn } from "@/shared/ui/class-name";

export type UiCheckboxSize = "default" | "small";

interface UiCheckboxProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  checkboxSize?: UiCheckboxSize;
  indeterminate?: boolean;
}

const CHECKBOX_SIZE_CLASS_MAP: Record<UiCheckboxSize, string> = {
  default: "h-4 w-4",
  small: "h-3.5 w-3.5",
};

export const UiCheckbox = forwardRef<HTMLInputElement, UiCheckboxProps>(function UiCheckbox(
  {
    "aria-checked": ariaChecked,
    checkboxSize = "default",
    className,
    indeterminate = false,
    ...props
  },
  ref,
) {
  const inputRef = useRef<HTMLInputElement>(null);
  useImperativeHandle(ref, () => inputRef.current as HTMLInputElement);
  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.indeterminate = indeterminate;
    }
  }, [indeterminate]);

  return (
    <input
      ref={inputRef}
      aria-checked={indeterminate ? "mixed" : ariaChecked}
      className={cn(
        "shrink-0 accent-(--primary) disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity) focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-[color:var(--ring)] focus-visible:ring-offset-1",
        CHECKBOX_SIZE_CLASS_MAP[checkboxSize],
        className,
      )}
      type="checkbox"
      {...props}
    />
  );
});
