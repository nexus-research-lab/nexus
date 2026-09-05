// INPUT: 原生输入属性、内容角色、字段描述/错误与搜索值变更命令。
// OUTPUT: 统一输入外观、原生校验反馈和可访问搜索清除行为。
// POS: 文本表单控件原语；不持有业务草稿、提交事务或领域校验规则。
"use client";

import {
  type ChangeEvent,
  type FormEvent,
  type InputHTMLAttributes,
  type ReactNode,
  type SelectHTMLAttributes,
  type TextareaHTMLAttributes,
  forwardRef,
  useId,
  useRef,
  useState,
} from "react";
import { Search, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  getUiFormControlClassName,
  getUiSearchInputShellClassName,
  type UiFormControlSize,
  type UiFormControlTextRole,
  type UiFormControlVariant,
  type UiSearchInputVariant,
} from "@/shared/ui/form/form-control-styles";

export type {
  UiFormControlSize,
  UiFormControlTextRole,
  UiFormControlVariant,
  UiSearchInputVariant,
} from "@/shared/ui/form/form-control-styles";

interface UiFieldProps {
  children: ReactNode;
  className?: string;
  description?: ReactNode;
  error?: ReactNode;
  htmlFor?: string;
  label?: ReactNode;
  labelClassName?: string;
  required?: boolean;
}

interface UiInputProps extends InputHTMLAttributes<HTMLInputElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  textRole?: UiFormControlTextRole;
  variant?: UiFormControlVariant;
}

interface UiNativeSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  textRole?: Exclude<UiFormControlTextRole, "verification">;
  variant?: UiFormControlVariant;
}

interface UiSearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "size" | "type" | "value"> {
  action?: ReactNode;
  className?: string;
  controlSize?: UiFormControlSize;
  inputClassName?: string;
  onChange: (value: string) => void;
  value: string;
  variant?: UiSearchInputVariant;
}

function findFirstInvalidControl(form: HTMLFormElement | null) {
  if (!form) {
    return null;
  }
  return Array.from(form.elements).find((element) => (
    element instanceof HTMLInputElement
    || element instanceof HTMLSelectElement
    || element instanceof HTMLTextAreaElement
  ) && !element.validity.valid) as
    | HTMLInputElement
    | HTMLSelectElement
    | HTMLTextAreaElement
    | undefined;
}

export function UiField({
  children,
  className: className,
  description,
  error,
  htmlFor: htmlFor,
  label,
  labelClassName,
  required = false,
}: UiFieldProps) {
  const { t } = useI18n();
  const errorId = useId();
  const invalidTargetRef = useRef<
    HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | null
  >(null);
  const [nativeError, setNativeError] = useState<string | null>(null);
  const labelError = label && !error ? nativeError : null;
  const contentError = error ?? (!label ? nativeError : null);

  const clearNativeError = () => {
    invalidTargetRef.current?.removeAttribute("aria-errormessage");
    invalidTargetRef.current?.removeAttribute("aria-invalid");
    invalidTargetRef.current = null;
    setNativeError(null);
  };

  const handleInvalid = (event: FormEvent<HTMLDivElement>) => {
    event.preventDefault();

    const target = event.target as
      | HTMLInputElement
      | HTMLSelectElement
      | HTMLTextAreaElement;
    const firstInvalid = findFirstInvalidControl(target.form);
    if (firstInvalid && firstInvalid !== target) {
      return;
    }

    clearNativeError();
    invalidTargetRef.current = target;
    target.setAttribute("aria-errormessage", errorId);
    target.setAttribute("aria-invalid", "true");
    setNativeError(
      t(target.validity.valueMissing ? "common.required_field" : "common.invalid_field"),
    );
    target.focus();
  };

  const handleInput = (event: FormEvent<HTMLDivElement>) => {
    const target = event.target as
      | HTMLInputElement
      | HTMLSelectElement
      | HTMLTextAreaElement;
    if (target !== invalidTargetRef.current) {
      return;
    }
    if (!target.validity.valid) {
      setNativeError(
        t(target.validity.valueMissing ? "common.required_field" : "common.invalid_field"),
      );
      return;
    }
    clearNativeError();
  };

  return (
    <div
      className={cn("dialog-field", className)}
      onInputCapture={handleInput}
      onInvalid={handleInvalid}
    >
      {label ? (
        <div className="flex min-h-5 items-center justify-between gap-2">
          <label className={cn("dialog-label min-w-0", labelClassName)} htmlFor={htmlFor}>
            {label}
            {required ? (
              <span aria-hidden="true" className="ml-0.5 text-(--destructive)">
                *
              </span>
            ) : null}
          </label>
          {labelError ? (
            <span
              className="shrink-0 text-xs leading-5 text-(--destructive)"
              id={errorId}
              role="alert"
            >
              {labelError}
            </span>
          ) : null}
        </div>
      ) : null}
      {children}
      {contentError ? (
        <p
          className="mt-2 text-xs leading-5 text-(--destructive)"
          id={errorId}
          role="alert"
        >
          {contentError}
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
    textRole,
    variant,
    ...props
  },
  ref,
) {
  return (
    <input
      ref={ref}
      className={getUiFormControlClassName(
        { size: controlSize, textRole, variant },
        cn(className),
      )}
      type={type}
      {...props}
    />
  );
});

export const UiNativeSelect = forwardRef<HTMLSelectElement, UiNativeSelectProps>(
  function UiNativeSelect(
    {
      className,
      controlSize,
      variant,
      ...props
    },
    ref,
  ) {
    return (
      <select
        ref={ref}
        className={getUiFormControlClassName(
          { size: controlSize, variant },
          cn(className),
        )}
        {...props}
      />
    );
  },
);

export const UiTextarea = forwardRef<HTMLTextAreaElement, UiTextareaProps>(function UiTextarea(
  {
    className,
    controlSize: controlSize,
    textRole,
    variant,
    ...props
  },
  ref,
) {
  return (
    <textarea
      ref={ref}
      className={getUiFormControlClassName(
        { multiline: true, size: controlSize, textRole, variant },
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
    <div
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
        aria-label={props["aria-label"] ?? placeholder}
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
        <UiIconButton
          aria-label={t("common.clear")}
          disabled={disabled || readOnly}
          onClick={(event) => {
            event.preventDefault();
            onChange("");
          }}
          onMouseDown={(event) => event.preventDefault()}
          size="xs"
          tooltip={t("common.clear")}
        >
          <X className="h-3.5 w-3.5" />
        </UiIconButton>
      ) : null}
      {action}
    </div>
  );
});
