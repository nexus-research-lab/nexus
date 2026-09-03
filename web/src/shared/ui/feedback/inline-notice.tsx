// INPUT: 已由业务确认的行内状态图标、标题、影响说明、tone 与至多一个恢复动作。
// OUTPUT: 使用统一 surface、排版和 Button 状态的 contained / edge 行内提示。
// POS: shared/ui 行内反馈视觉与 DOM 所有者；不分类失败、不推测恢复方式或自动执行动作。
"use client";

import type { HTMLAttributes, ReactNode } from "react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type UiInlineNoticeTone = "neutral" | "warning" | "danger";
export type UiInlineNoticeVariant = "contained" | "edge";

interface UiInlineNoticeAction {
  disabled?: boolean;
  icon?: ReactNode;
  label: ReactNode;
  onClick: () => void;
  pending?: boolean;
}

interface UiInlineNoticeProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  action?: UiInlineNoticeAction;
  icon?: ReactNode;
  message: ReactNode;
  title?: ReactNode;
  tone?: UiInlineNoticeTone;
  variant?: UiInlineNoticeVariant;
}

const TONE_CLASS_NAMES: Record<UiInlineNoticeTone, {
  background: string;
  icon: string;
}> = {
  neutral: {
    background: "bg-(--surface-control-background)",
    icon: "text-(--icon-muted)",
  },
  warning: {
    background: "bg-[color:color-mix(in_srgb,var(--warning)_2%,var(--surface-control-background))]",
    icon: "text-(--warning)",
  },
  danger: {
    background: "bg-[color:color-mix(in_srgb,var(--destructive)_2%,var(--surface-control-background))]",
    icon: "text-(--destructive)",
  },
};

export function UiInlineNotice({
  action,
  "aria-live": ariaLive = "polite",
  className,
  icon,
  message,
  role = "status",
  title,
  tone = "neutral",
  variant = "contained",
  ...props
}: UiInlineNoticeProps) {
  const toneClassNames = TONE_CLASS_NAMES[tone];
  return (
    <div
      aria-atomic="true"
      aria-live={ariaLive}
      className={cn(
        "flex min-w-0 items-start gap-x-2.5 gap-y-1 text-(--text-muted)",
        variant === "contained"
          ? "min-h-8 w-full surface-radius-sm border border-(--surface-control-border) px-2.5 py-1.5"
          : "flex-wrap border-y border-(--divider-subtle-color) px-3 py-2 sm:flex-nowrap",
        toneClassNames.background,
        className,
      )}
      data-inline-notice-tone={tone}
      data-inline-notice-variant={variant}
      role={role}
      {...props}
    >
      {icon ? (
        <span
          aria-hidden="true"
          className={cn(
            "mt-0.5 grid h-3.5 w-3.5 shrink-0 place-items-center [&>svg]:h-3.5 [&>svg]:w-3.5",
            toneClassNames.icon,
          )}
          data-inline-notice-icon
        >
          {icon}
        </span>
      ) : null}
      <div
        className={cn(
          variant === "edge"
            ? "w-[calc(100%-1.5rem)] min-w-0 flex-none sm:w-auto sm:flex-1"
            : "min-w-0 flex-1",
        )}
      >
        {title ? (
          <p
            className={cn(
              "break-words leading-5 [overflow-wrap:anywhere]",
              getUiTypographyClassName({
                role: "metadata",
                tone: "strong",
                weight: "medium",
              }),
            )}
            data-inline-notice-title
          >
            {title}
          </p>
        ) : null}
        <div
          className={cn(
            "break-words leading-5 [overflow-wrap:anywhere]",
            getUiTypographyClassName({ role: "metadata", tone: "muted" }),
            title && "mt-0.5",
          )}
          data-inline-notice-message
        >
          {message}
        </div>
      </div>
      {action ? (
        <UiButton
          aria-busy={action.pending || undefined}
          className={cn(
            "shrink-0 self-start px-1.5 motion-reduce:transition-none",
            variant === "edge" && "ml-6 sm:ml-0",
          )}
          disabled={Boolean(action.disabled || action.pending)}
          onClick={action.onClick}
          size="xs"
          tone="primary"
          variant="text"
        >
          {action.icon ? (
            <span
              aria-hidden="true"
              className={cn(
                "grid h-3.5 w-3.5 shrink-0 place-items-center [&>svg]:h-3.5 [&>svg]:w-3.5",
                action.pending && getUiSpinnerClassName({ size: "sm" }),
              )}
              data-inline-notice-action-icon
            >
              {action.icon}
            </span>
          ) : null}
          {action.label}
        </UiButton>
      ) : null}
    </div>
  );
}
