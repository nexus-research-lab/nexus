"use client";

import {
  type FormHTMLAttributes,
  type HTMLAttributes,
  type ReactNode,
  type RefObject,
  useRef,
} from "react";
import { createPortal } from "react-dom";
import { X } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { useDialogModalBehavior } from "@/shared/ui/dialog/dialog-behavior";
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
  className?: string;
  labelledBy?: string;
  describedBy?: string;
  initialFocusRef?: RefObject<HTMLElement | null>;
  onClose?: () => void;
  trapFocus?: boolean;
}

interface UiDialogShellProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  className?: string;
  size?: UiDialogSize;
}

interface UiDialogFormShellProps extends FormHTMLAttributes<HTMLFormElement> {
  children: ReactNode;
  className?: string;
  size?: UiDialogSize;
}

interface UiDialogHeaderProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
  icon?: ReactNode;
  iconClassName?: string;
  onClose?: () => void;
  subtitle?: ReactNode;
  title?: ReactNode;
  titleId?: string;
}

interface UiDialogBodyProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  className?: string;
  scrollable?: boolean;
}

interface UiDialogFooterProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  className?: string;
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
  className,
  describedBy: describedBy,
  initialFocusRef: initialFocusRef,
  labelledBy: labelledBy,
  onClick,
  onClose: onClose,
  trapFocus: trapFocus = true,
  ...props
}: UiDialogBackdropProps) {
  const rootRef = useRef<HTMLDivElement | null>(null);
  useDialogModalBehavior({
    enabled: trapFocus,
    initialFocusRef,
    onClose,
    rootRef,
  });

  return (
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions -- backdrop click-to-close + Escape is a standard modal dialog pattern
    <div
      ref={rootRef}
      aria-describedby={describedBy}
      aria-labelledby={labelledBy}
      aria-modal="true"
      className={cn(DIALOG_BACKDROP_CLASS_NAME, className)}
      data-modal-root="true"
      data-ui-dialog-root="true"
      onClick={(event) => {
        onClick?.(event);
        if (!event.defaultPrevented && event.target === event.currentTarget) {
          onClose?.();
        }
      }}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          onClose?.();
        }
      }}
      role="dialog"
      tabIndex={-1}
      {...props}
    >
      {children}
    </div>
  );
}

export function UiDialogShell({
  children,
  className,
  size = "md",
  ...props
}: UiDialogShellProps) {
  return (
    <section
      className={cn(
        "dialog-shell surface-radius-md flex w-full flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)",
        DIALOG_SIZE_CLASS_MAP[size],
        className,
      )}
      tabIndex={-1}
      {...props}
    >
      {children}
    </section>
  );
}

export function UiDialogFormShell({
  children,
  className,
  size = "md",
  ...props
}: UiDialogFormShellProps) {
  return (
    <form
      className={cn(
        "dialog-shell surface-radius-md flex w-full flex-col overflow-hidden animate-in zoom-in-95 duration-(--motion-duration-fast)",
        DIALOG_SIZE_CLASS_MAP[size],
        className,
      )}
      tabIndex={-1}
      {...props}
    >
      {children}
    </form>
  );
}

export function UiDialogHeader({
  actions,
  children,
  className,
  icon,
  iconClassName: iconClassName,
  onClose: onClose,
  subtitle,
  title,
  titleId: titleId,
  ...props
}: UiDialogHeaderProps) {
  return (
    <div className={cn("dialog-header", className)} {...props}>
      {children ?? (
        <div className={cn(DIALOG_HEADER_LEADING_CLASS_NAME, "min-w-0 flex-1 items-center")}>
          {icon ? (
            <div className={cn(DIALOG_HEADER_ICON_CLASS_NAME, iconClassName)}>
              {icon}
            </div>
          ) : null}
          <div className="min-w-0 flex-1">
            {title ? (
              <h2 className="dialog-title" id={titleId}>
                {title}
              </h2>
            ) : null}
            {subtitle ? <p className="dialog-subtitle">{subtitle}</p> : null}
          </div>
        </div>
      )}
      {actions}
      {onClose ? <UiDialogCloseButton onClose={onClose} /> : null}
    </div>
  );
}

export function UiDialogBody({
  children,
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
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function UiDialogFooter({
  children,
  className,
  ...props
}: UiDialogFooterProps) {
  return (
    <div className={cn("dialog-footer", className)} {...props}>
      {children}
    </div>
  );
}

export function UiDialogCloseButton({
  className,
  onClose: onClose,
}: {
  className?: string;
  onClose: () => void;
}) {
  return (
    <button
      aria-label="关闭"
      className={cn(DIALOG_ICON_BUTTON_CLASS_NAME, className)}
      onClick={(event) => {
        event.preventDefault();
        event.stopPropagation();
        onClose();
      }}
      onPointerDown={(event) => {
        event.stopPropagation();
      }}
      type="button"
    >
      <X className="h-4 w-4" />
    </button>
  );
}
