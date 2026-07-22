"use client";

import {
  type ChangeEvent,
  type InputHTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
  type TextareaHTMLAttributes,
  forwardRef,
  useState,
} from "react";
import { Search } from "lucide-react";

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

interface UiPasswordInputProps extends Omit<UiInputProps, "type"> {
  containerClassName?: string;
}

interface UiTextareaProps extends TextareaHTMLAttributes<HTMLTextAreaElement> {
  className?: string;
  controlSize?: UiFormControlSize;
  variant?: UiFormControlVariant;
}

interface UiSearchInputProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "onChange" | "size"> {
  action?: ReactNode;
  className?: string;
  controlSize?: UiFormControlSize;
  inputClassName?: string;
  onChange: (value: string) => void;
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

/** 密码输入统一接管 Caps Lock 提示，避免宿主 WebKit 指示器随控件尺寸异常缩放。 */
export const UiPasswordInput = forwardRef<HTMLInputElement, UiPasswordInputProps>(
  function UiPasswordInput(
    {
      className,
      containerClassName,
      onBlur,
      onKeyDown,
      onKeyUp,
      ...props
    },
    ref,
  ) {
    const { t } = useI18n();
    const [capsLockOn, setCapsLockOn] = useState(false);
    const syncCapsLock = (event: KeyboardEvent<HTMLInputElement>) => {
      setCapsLockOn(event.getModifierState("CapsLock"));
    };

    return (
      <span className={cn("relative block min-w-0", containerClassName)}>
        <UiInput
          ref={ref}
          className={cn(className, "nexus-password-input pr-11")}
          onBlur={(event) => {
            setCapsLockOn(false);
            onBlur?.(event);
          }}
          onKeyDown={(event) => {
            syncCapsLock(event);
            onKeyDown?.(event);
          }}
          onKeyUp={(event) => {
            syncCapsLock(event);
            onKeyUp?.(event);
          }}
          type="password"
          {...props}
        />
        {capsLockOn ? (
          <span
            aria-hidden="true"
            className="pointer-events-none absolute right-2.5 top-1/2 inline-flex h-6 min-w-6 -translate-y-1/2 items-center justify-center rounded-[8px] border border-(--divider-strong-color) bg-(--surface-card-background) px-1.5 text-[11px] font-semibold leading-none text-(--text-muted) shadow-[var(--surface-inset-shadow)]"
            title={t("common.caps_lock_on")}
          >
            ⇧
          </span>
        ) : null}
        <span aria-live="polite" className="sr-only">
          {capsLockOn ? t("common.caps_lock_on") : ""}
        </span>
      </span>
    );
  },
);

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
  inputClassName: inputClassName,
  onChange: onChange,
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
