"use client";

import {
  type FormHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";
import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";

type UiDialogSize = "sm" | "md" | "lg" | "xl" | "wide";

const DIALOG_SIZE_CLASS_MAP: Record<UiDialogSize, string> = {
  sm: "max-w-md",
  md: "max-w-lg",
  lg: "max-w-2xl",
  xl: "max-w-4xl",
  wide: "max-w-5xl",
};

interface UiDialogPortalProps {
  children: ReactNode;
}

interface UiDialogBackdropProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  class_name?: string;
  labelled_by?: string;
  described_by?: string;
  on_close?: () => void;
}

interface UiDialogShellProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  class_name?: string;
  size?: UiDialogSize;
}

interface UiDialogFormShellProps extends FormHTMLAttributes<HTMLFormElement> {
  children: ReactNode;
  class_name?: string;
  size?: UiDialogSize;
}

interface UiDialogHeaderProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  children?: ReactNode;
  class_name?: string;
  icon?: ReactNode;
  icon_class_name?: string;
  on_close?: () => void;
  subtitle?: ReactNode;
  title?: ReactNode;
  title_id?: string;
}

interface UiDialogBodyProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  class_name?: string;
  scrollable?: boolean;
}

interface UiDialogFooterProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  class_name?: string;
}

export function UiDialogPortal({ children }: UiDialogPortalProps) {
  if (typeof document === "undefined") {
    return null;
  }

  return createPortal(children, document.body);
}

/** 中文注释：弹窗骨架统一处理遮罩点击，避免业务弹窗各写一套事件判断。 */
export function UiDialogBackdrop({
  children,
  class_name,
  className,
  described_by,
  labelled_by,
  onClick,
  on_close,
  ...props
}: UiDialogBackdropProps) {
  return (
    <div
      aria-describedby={described_by}
      aria-labelledby={labelled_by}
      aria-modal="true"
      className={cn(DIALOG_BACKDROP_CLASS_NAME, className, class_name)}
      data-modal-root="true"
      onClick={(event) => {
        onClick?.(event);
        if (!event.defaultPrevented && event.target === event.currentTarget) {
          on_close?.();
        }
      }}
      role="dialog"
      {...props}
    >
      {children}
    </div>
  );
}

export function UiDialogShell({
  children,
  class_name,
  className,
  size = "md",
  ...props
}: UiDialogShellProps) {
  return (
    <section
      className={cn(
        "dialog-shell radius-shell-xl flex w-full flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)",
        DIALOG_SIZE_CLASS_MAP[size],
        className,
        class_name,
      )}
      {...props}
    >
      {children}
    </section>
  );
}

export function UiDialogFormShell({
  children,
  class_name,
  className,
  size = "md",
  ...props
}: UiDialogFormShellProps) {
  return (
    <form
      className={cn(
        "dialog-shell radius-shell-xl flex w-full flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)",
        DIALOG_SIZE_CLASS_MAP[size],
        className,
        class_name,
      )}
      {...props}
    >
      {children}
    </form>
  );
}

export function UiDialogHeader({
  actions,
  children,
  class_name,
  className,
  icon,
  icon_class_name,
  on_close,
  subtitle,
  title,
  title_id,
  ...props
}: UiDialogHeaderProps) {
  return (
    <div className={cn("dialog-header", className, class_name)} {...props}>
      {children ?? (
        <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
          {icon ? (
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, icon_class_name)}>
              {icon}
            </div>
          ) : null}
          <div className="min-w-0 flex-1">
            {title ? (
              <h2 className="dialog-title" id={title_id}>
                {title}
              </h2>
            ) : null}
            {subtitle ? <p className="dialog-subtitle">{subtitle}</p> : null}
          </div>
        </div>
      )}
      {actions}
      {on_close ? <UiDialogCloseButton on_close={on_close} /> : null}
    </div>
  );
}

export function UiDialogBody({
  children,
  class_name,
  className,
  scrollable = false,
  ...props
}: UiDialogBodyProps) {
  return (
    <div
      className={cn(
        "dialog-body",
        scrollable && "dialog-body--scroll soft-scrollbar min-h-0 flex-1",
        className,
        class_name,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function UiDialogFooter({
  children,
  class_name,
  className,
  ...props
}: UiDialogFooterProps) {
  return (
    <div className={cn("dialog-footer", className, class_name)} {...props}>
      {children}
    </div>
  );
}

export function UiDialogCloseButton({
  class_name,
  className,
  on_close,
}: {
  class_name?: string;
  className?: string;
  on_close: () => void;
}) {
  return (
    <button
      aria-label="关闭"
      className={cn(DIALOG_ICON_BUTTON_CLASS_NAME, className, class_name)}
      onClick={on_close}
      type="button"
    >
      <X className="h-4 w-4" />
    </button>
  );
}
