"use client";

import {
  type ChangeEvent,
  type InputHTMLAttributes,
  type ReactNode,
  type TextareaHTMLAttributes,
  forwardRef,
} from "react";
import { Search, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import {
  getUiFormControlClassName,
  getUiSearchInputShellClassName,
  type UiFormControlSize,
  type UiFormControlVariant,
} from "@/shared/ui/form/form-control-styles";

interface UiFieldProps {
  children: ReactNode;
  className?: string;
  description?: ReactNode;
  error?: ReactNode;
  htmlFor?: string;
  label?: ReactNode;
}

interface UiInputProps extends InputHTMLAttributes<HTMLInputElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiSearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "size" | "type" | "value"> {
  action?: ReactNode;
  className?: string;
  controlSize?: UiFormControlSize;
  inputClassName?: string;
  onChange: (value: string) => void;
  value: string;
  variant?: UiFormControlVariant;
}

export function UiField({
  children,
  className: className,
  description,
  error,
  htmlFor: htmlFor,
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
    className,
    controlSize: controlSize,
    type = "text",
    variant,
    ...props
  },
  ref,
) {
  return (
    <input
      ref={ref}
      className={getUiFormControlClassName(
        { size: controlSize, variant },
        cn(className),
      )}
      type={type}
      {...props}
    />
  );
});

export const UiTextarea = forwardRef<HTMLTextAreaElement, UiTextareaProps>(function UiTextarea(
  {
    className,
    controlSize: controlSize,
    variant,
    ...props
  },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={getUiFormControlClassName(
        { multiline: true, size: controlSize, variant },
        cn("resize-y", className),
      )}
      {...props}
    />
  );
});

export const UiSearchInput = forwardRef<HTMLInputElement, UiSearchInputProps>(function UiSearchInput({
  action,
  className,
  controlSize: controlSize,
  disabled,
  inputClassName: inputClassName,
  onChange: onChange,
  placeholder = "搜索",
  readOnly,
  value,
  variant,
  ...props
}: UiSearchInputProps, ref) {
  const { t } = useI18n();
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange(event.target.value);
  };

  return (
    <label
      className={getUiSearchInputShellClassName(
        { size: controlSize, variant },
        cn(className),
      )}
    >
      <Search className="h-4 w-4 shrink-0 text-(--icon-default)" />
      <input
        className={cn(
          "min-w-0 flex-1 bg-transparent text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-soft) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
          inputClassName,
        )}
        disabled={disabled}
        onChange={handleChange}
        placeholder={placeholder}
        readOnly={readOnly}
        role="searchbox"
        type="text"
        value={value}
        ref={ref}
        {...props}
      />
      {value ? (
        <button
          aria-label={t("common.clear")}
          className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-[6px] text-(--icon-default) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-default) disabled:pointer-events-none disabled:opacity-45"
          disabled={disabled || readOnly}
          onClick={(event) => {
            event.preventDefault();
            onChange("");
          }}
          onMouseDown={(event) => event.preventDefault()}
          title={t("common.clear")}
          type="button"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      ) : null}
      {action}
    </label>
  );
});
