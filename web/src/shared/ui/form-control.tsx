"use client";

import {
  type ChangeEvent,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
  forwardRef,
} from "react";
import { Search } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  get_ui_form_control_class_name,
  get_ui_search_input_shell_class_name,
  type UiFormControlSize,
  type UiFormControlVariant,
} from "@/shared/ui/form-control-styles";

interface UiFieldProps {
  children: ReactNode;
  class_name?: string;
  description?: ReactNode;
  error?: ReactNode;
  html_for?: string;
  label?: ReactNode;
}

interface UiInputProps extends InputHTMLAttributes<HTMLInputElement> {
  class_name?: string;
  control_size?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  class_name?: string;
  control_size?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiSearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "size"> {
  action?: ReactNode;
  class_name?: string;
  control_size?: UiFormControlSize;
  input_class_name?: string;
  on_change: (value: string) => void;
  variant?: UiFormControlVariant;
}

export function UiField({
  children,
  class_name: className,
  description,
  error,
  html_for: htmlFor,
  label,
}: UiFieldProps) {
  return (
    <div className={cn("dialog-field", className)}>
      {label ? (
        <label className="dialog-label" htmlFor={htmlFor}>
          {label}
        </label>
      ) : null}
      {children}
      {error ? (
        <p className="mt-2 text-xs leading-5 text-(--destructive)">
          {error}
        </p>
      ) : description ? (
        <p className="mt-2 text-xs leading-5 text-(--text-muted)">
          {description}
        </p>
      ) : null}
    </div>
  );
}

export const UiInput = forwardRef<HTMLInputElement, UiInputProps>(function UiInput(
  {
    class_name: legacyClassName,
    className,
    control_size: controlSize,
    type = "text",
    variant,
    ...props
  },
  ref,
) {
  return (
    <input
      ref={ref}
      className={get_ui_form_control_class_name(
        { size: controlSize, variant },
        cn(className, legacyClassName),
      )}
      type={type}
      {...props}
    />
  );
});

export const UiTextarea = forwardRef<HTMLTextAreaElement, UiTextareaProps>(function UiTextarea(
  {
    class_name: legacyClassName,
    className,
    control_size: controlSize,
    variant,
    ...props
  },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={get_ui_form_control_class_name(
        { multiline: true, size: controlSize, variant },
        cn("resize-y", className, legacyClassName),
      )}
      {...props}
    />
  );
});

export const UiSearchInput = forwardRef<HTMLInputElement, UiSearchInputProps>(function UiSearchInput({
  action,
  class_name: legacyClassName,
  className,
  control_size: controlSize,
  input_class_name: inputClassName,
  on_change: onChange,
  placeholder = "搜索",
  type,
  value,
  variant,
  ...props
}: UiSearchInputProps, ref) {
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange(event.target.value);
  };

  return (
    <label
      className={get_ui_search_input_shell_class_name(
        { size: controlSize, variant },
        cn(className, legacyClassName),
      )}
    >
      <Search className="h-4 w-4 shrink-0 text-(--icon-default)" />
      <input
        className={cn(
          "min-w-0 flex-1 bg-transparent text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
          inputClassName,
        )}
        onChange={handleChange}
        placeholder={placeholder}
        type={type ?? "search"}
        value={value}
        ref={ref}
        {...props}
      />
      {action}
    </label>
  );
});
