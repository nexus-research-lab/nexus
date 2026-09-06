// INPUT: 原生输入属性、内容角色、字段描述/错误与搜索值变更命令。
// OUTPUT: 统一输入外观、精确关联的标签/说明/错误、原生校验反馈和可访问搜索清除行为。
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
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Search, X } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { FIELD_ACCESSIBILITY_CONTEXT, useFieldControlAttributes } from "./field-accessibility";
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
  ) && element.willValidate && !element.validity.valid) as
    | HTMLInputElement
    | HTMLSelectElement
    | HTMLTextAreaElement
    | undefined;
}

export function UiField({
  children,
  className,
  description,
  error,
  htmlFor,
  label,
  labelClassName,
  required = false,
}: UiFieldProps) {
  const { t } = useI18n();
  const errorId = useId();
  const descriptionId = useId();
  const labelId = useId();
  const isGroup = !htmlFor && Boolean(label);
  const Label = htmlFor ? "label" : "span";
  const invalidTargetRef = useRef<
    HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement | null
  >(null);
  const [nativeError, setNativeError] = useState<{ controlId: string; message: string } | null>(null);
  const labelError = label && !error ? nativeError?.message : null;
  const contentError = error ?? (!label ? nativeError?.message : null);
  const visibleDescriptionId = description && !contentError ? descriptionId : undefined;

  const clearNativeError = () => {
    invalidTargetRef.current = null;
    setNativeError(null);
  };

  // 值被业务重置、控件被移除或禁用时，旧原生错误不能继续覆盖当前属性。
  useLayoutEffect(() => {
    const target = invalidTargetRef.current;
    if (target && (!target.isConnected || !target.willValidate || target.validity.valid)) {
      clearNativeError();
    }
  });

  const handleInvalid = (event: FormEvent<HTMLDivElement>) => {
    const target = event.target as
      | HTMLInputElement
      | HTMLSelectElement
      | HTMLTextAreaElement;
    if (target.closest("[data-ui-field]") !== event.currentTarget) {
      return;
    }
    event.preventDefault();
    const firstInvalid = findFirstInvalidControl(target.form);
    if (firstInvalid && firstInvalid !== target) {
      return;
    }

    invalidTargetRef.current = target;
    setNativeError({
      controlId: target.id,
      message: t(target.validity.valueMissing ? "common.required_field" : "common.invalid_field"),
    });
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
      setNativeError({
        controlId: target.id,
        message: t(target.validity.valueMissing ? "common.required_field" : "common.invalid_field"),
      });
      return;
    }
    clearNativeError();
  };

  return (
    <FIELD_ACCESSIBILITY_CONTEXT.Provider value={{
      controlId: htmlFor,
      descriptionId: visibleDescriptionId,
      errorId,
      hasError: Boolean(error),
      nativeInvalidControlId: nativeError?.controlId,
    }}>
      <div
        aria-describedby={isGroup ? visibleDescriptionId : undefined}
        aria-errormessage={isGroup && error ? errorId : undefined}
        aria-invalid={isGroup && error ? true : undefined}
        aria-labelledby={isGroup ? labelId : undefined}
        className={cn("dialog-field", className)}
        data-ui-field=""
        onInputCapture={handleInput}
        onInvalid={handleInvalid}
        role={isGroup ? "group" : undefined}
      >
        {label ? (
          <div className="flex min-h-5 items-center justify-between gap-2">
            <Label className={cn("dialog-label min-w-0", labelClassName)} htmlFor={htmlFor} id={labelId}>
              {label}
              {required ? (
                <span aria-hidden="true" className="ml-0.5 text-(--destructive)">
                  *
                </span>
              ) : null}
            </Label>
            {labelError ? (
              <span
                className={cn("shrink-0", getUiTypographyClassName({ role: "metadata", tone: "danger" }))}
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
            className={getUiTypographyClassName({ role: "supporting", tone: "danger" })}
            id={errorId}
            role="alert"
          >
            {contentError}
          </p>
        ) : description ? (
          <p className={getUiTypographyClassName({ role: "supporting", tone: "muted" })} id={descriptionId}>
            {description}
          </p>
        ) : null}
      </div>
    </FIELD_ACCESSIBILITY_CONTEXT.Provider>
  );
}

export const UiInput = forwardRef<HTMLInputElement, UiInputProps>(function UiInput(
  {
    className,
    controlSize,
    type = "text",
    textRole,
    variant,
    ...props
  },
  ref,
) {
  const fieldAttributes = useFieldControlAttributes(props);
  return (
    <input
      ref={ref}
      className={getUiFormControlClassName(
        { size: controlSize, textRole, variant },
        cn(className),
      )}
      type={type}
      {...props}
      {...fieldAttributes}
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
    const fieldAttributes = useFieldControlAttributes(props);
    return (
      <select
        ref={ref}
        className={getUiFormControlClassName(
          { size: controlSize, variant },
          cn(className),
        )}
        {...props}
        {...fieldAttributes}
      />
    );
  },
);

export const UiTextarea = forwardRef<HTMLTextAreaElement, UiTextareaProps>(function UiTextarea(
  {
    className,
    controlSize,
    textRole,
    variant,
    ...props
  },
  ref,
) {
  const fieldAttributes = useFieldControlAttributes(props);
  return (
    <textarea
      ref={ref}
      className={getUiFormControlClassName(
        { multiline: true, size: controlSize, textRole, variant },
        cn("resize-y", className),
      )}
      {...props}
      {...fieldAttributes}
    />
  );
});

export const UiSearchInput = forwardRef<HTMLInputElement, UiSearchInputProps>(function UiSearchInput({
  action,
  className,
  controlSize,
  disabled,
  inputClassName,
  onChange,
  placeholder = "搜索",
  readOnly,
  value,
  variant,
  ...props
}: UiSearchInputProps, ref) {
  const { t } = useI18n();
  const fieldAttributes = useFieldControlAttributes(props);
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
          "min-w-0 flex-1 bg-transparent text-(--text-strong) outline-none shadow-none ring-0 placeholder:text-(--text-muted) focus:outline-none focus:ring-0 focus-visible:outline-none focus-visible:ring-0 focus-visible:shadow-none",
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
        {...fieldAttributes}
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
