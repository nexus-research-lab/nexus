// INPUT: 业务弹窗提供标题、正文、动作与可选的默认或 plain chrome。
// OUTPUT: 统一的可访问模态骨架与可禁用关闭动作；plain chrome 用于连接、授权与紧凑表单。
// POS: Web 共享弹窗结构真相源，业务层只选择语义密度，不自行重写遮罩、焦点与关闭协议。
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

import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { useDialogModalBehavior } from "@/shared/ui/dialog/dialog-behavior";
import {
  getUiDialogViewportClassName,
  type UiDialogViewport,
} from "@/shared/ui/dialog/dialog-layout";
import {
  DIALOG_BACKDROP_CLASS_NAME,
  DIALOG_HEADER_ICON_CLASS_NAME,
  DIALOG_HEADER_LEADING_CLASS_NAME,
  DIALOG_ICON_BUTTON_CLASS_NAME,
} from "@/shared/ui/dialog/dialog-styles";
import {
  getUiOverlayLayerClassName,
  type UiDialogLayer,
} from "@/shared/ui/overlay/layer-styles";

export type UiDialogSize = "xs" | "sm" | "md" | "lg" | "xl" | "wide" | "workbench";
type UiDialogChrome = "default" | "plain";

const DIALOG_SIZE_CLASS_MAP: Record<UiDialogSize, string> = {
  xs: "max-w-sm",
  sm: "max-w-md",
  md: "max-w-lg",
  lg: "max-w-2xl",
  xl: "max-w-4xl",
  wide: "max-w-5xl",
  workbench: "ui-dialog-size-workbench",
};

interface UiDialogPortalProps {
  children: ReactNode;
}

interface UiDialogBackdropProps extends HTMLAttributes<HTMLDivElement> {
  children: ReactNode;
  className?: string;
  closeOnBackdrop?: boolean;
  labelledBy?: string;
  describedBy?: string;
  initialFocusRef?: RefObject<HTMLElement | null>;
  inset?: "compact" | "default";
  layer?: UiDialogLayer;
  onClose?: () => void;
  trapFocus?: boolean;
}

interface UiDialogShellProps extends HTMLAttributes<HTMLElement> {
  children: ReactNode;
  className?: string;
  size?: UiDialogSize;
  viewport?: UiDialogViewport;
}

interface UiDialogFormShellProps extends FormHTMLAttributes<HTMLFormElement> {
  children: ReactNode;
  className?: string;
  size?: UiDialogSize;
  viewport?: UiDialogViewport;
}

interface UiDialogHeaderProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  appearance?: UiDialogChrome;
  children?: ReactNode;
  className?: string;
  closeLabel?: string;
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
  appearance?: UiDialogChrome;
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
  closeOnBackdrop = true,
  describedBy,
  initialFocusRef,
  inset = "default",
  labelledBy,
  layer,
  onClick,
  onClose,
  trapFocus = true,
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
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-noninteractive-element-interactions -- 模态根节点统一承载遮罩关闭，键盘协议由行为层监听。
    <div
      ref={rootRef}
      aria-describedby={describedBy}
      aria-labelledby={labelledBy}
      aria-modal="true"
      className={cn(
        DIALOG_BACKDROP_CLASS_NAME,
        inset === "compact" && "ui-dialog-backdrop-compact",
        getUiOverlayLayerClassName(layer),
        className,
      )}
      data-modal-root="true"
      data-ui-dialog-root="true"
      onClick={(event) => {
        onClick?.(event);
        if (
          closeOnBackdrop
          && !event.defaultPrevented
          && event.target === event.currentTarget
        ) {
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
  viewport = "content",
  ...props
}: UiDialogShellProps) {
  return (
    <section
      className={cn(
        "dialog-shell surface-radius-lg flex w-full flex-col overflow-hidden animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-normal)",
        DIALOG_SIZE_CLASS_MAP[size],
        getUiDialogViewportClassName(viewport),
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
  viewport = "content",
  ...props
}: UiDialogFormShellProps) {
  return (
    <form
      className={cn(
        "dialog-shell surface-radius-lg flex w-full flex-col overflow-hidden animate-in fade-in-0 zoom-in-95 duration-(--motion-duration-normal)",
        DIALOG_SIZE_CLASS_MAP[size],
        getUiDialogViewportClassName(viewport),
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
  appearance = "default",
  children,
  className,
  closeLabel,
  icon,
  iconClassName,
  onClose,
  subtitle,
  title,
  titleId,
  ...props
}: UiDialogHeaderProps) {
  return (
    <div
      className={cn(
        "dialog-header",
        appearance === "plain" && "dialog-header--plain",
        className,
      )}
      {...props}
    >
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
      {onClose ? <UiDialogCloseButton ariaLabel={closeLabel} onClose={onClose} /> : null}
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
  appearance = "default",
  children,
  className,
  ...props
}: UiDialogFooterProps) {
  return (
    <div
      className={cn(
        "dialog-footer",
        appearance === "plain" && "dialog-footer--plain",
        className,
      )}
      {...props}
    >
      {children}
    </div>
  );
}

export function UiDialogCloseButton({
  ariaLabel,
  className,
  disabled = false,
  onClose,
}: {
  ariaLabel?: string;
  className?: string;
  disabled?: boolean;
  onClose: () => void;
}) {
  const { t } = useI18n();
  return (
    <button
      aria-label={ariaLabel ?? t("common.close")}
      className={cn(DIALOG_ICON_BUTTON_CLASS_NAME, className)}
      disabled={disabled}
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
