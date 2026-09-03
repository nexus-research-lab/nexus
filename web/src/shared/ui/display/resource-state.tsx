// INPUT: 上层已经确定的加载、空、失败、决策或完成展示内容与可选动作。
// OUTPUT: 失败面最多一个安全恢复动作；需要双向选择的冲突使用独立 decision 态。
// POS: 纯展示组件；不判断 query、mutation、access、离线或重试语义。
"use client";

import type { HTMLAttributes, ReactNode } from "react";
import {
  CheckCircle2,
  CircleAlert,
  Inbox,
  LoaderCircle,
} from "lucide-react";

import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { RecoverySummary } from "@/shared/ui/feedback/recovery-summary";
import type {
  UiStateBlockSize,
  UiStateBlockVariant,
} from "@/shared/ui/display/state-block-styles";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

export type UiResourceStateKind =
  | "loading"
  | "empty"
  | "error"
  | "decision"
  | "success";

export interface UiResourceStateAction {
  busy?: boolean;
  busyLabel?: ReactNode;
  disabled?: boolean;
  icon?: ReactNode;
  label: ReactNode;
  onClick: () => void;
  tone?: "danger" | "primary";
}

interface UiResourceStateBaseProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  className?: string;
  icon?: ReactNode;
  size?: UiStateBlockSize;
  title: ReactNode;
  urgency?: "assertive" | "polite";
  variant?: UiStateBlockVariant;
}

interface UiResourceLoadingStateProps extends UiResourceStateBaseProps {
  description?: ReactNode;
  impact?: ReactNode;
  nextStep?: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "loading";
  tone?: never;
}

interface UiResourceEmptyStateProps extends UiResourceStateBaseProps {
  description?: ReactNode;
  impact?: ReactNode;
  nextStep?: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "empty";
  tone?: never;
}

interface UiResourceFailureStateProps extends UiResourceStateBaseProps {
  description?: never;
  impact: ReactNode;
  nextStep?: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: never;
  state: "error";
  tone?: "danger" | "warning";
}

interface UiResourceDecisionStateProps extends UiResourceStateBaseProps {
  description?: never;
  impact: ReactNode;
  nextStep?: ReactNode;
  primaryAction: UiResourceStateAction;
  secondaryAction: UiResourceStateAction;
  state: "decision";
  tone: "warning";
}

interface UiResourceSuccessStateProps extends UiResourceStateBaseProps {
  description?: ReactNode;
  impact?: ReactNode;
  nextStep?: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "success";
  tone?: never;
}

export type UiResourceStateProps =
  | UiResourceLoadingStateProps
  | UiResourceEmptyStateProps
  | UiResourceFailureStateProps
  | UiResourceDecisionStateProps
  | UiResourceSuccessStateProps;

const DEFAULT_STATE_ICONS: Record<UiResourceStateKind, ReactNode> = {
  empty: <Inbox className="h-5 w-5 text-(--icon-default)" />,
  error: <CircleAlert className="h-4 w-4 text-(--destructive)" />,
  decision: <CircleAlert className="h-4 w-4 text-(--warning)" />,
  loading: (
    <LoaderCircle
      aria-hidden
      className={getUiSpinnerClassName({ size: "lg", tone: "muted" })}
    />
  ),
  success: <CheckCircle2 className="h-5 w-5 text-(--success)" />,
};

export function UiResourceState({
  className,
  description,
  icon,
  impact,
  nextStep,
  primaryAction,
  secondaryAction,
  size,
  state,
  title,
  tone = "danger",
  urgency = "polite",
  variant,
  ...props
}: UiResourceStateProps) {
  const failure = state === "error";
  const decision = state === "decision";
  const recovery = failure || decision;
  const compactFailure = recovery && size === "sm";
  const compactState = compactFailure
    || (state === "success" && size === "sm" && variant === "card");
  const resolvedIcon = icon ?? (recovery && tone === "warning"
    ? <CircleAlert className="h-4 w-4 text-(--warning)" />
    : DEFAULT_STATE_ICONS[state]);

  return (
    <UiStateBlock
      aria-atomic="true"
      aria-live={urgency}
      aria-busy={state === "loading"}
      className={cn(className, compactState && "items-start text-left")}
      data-resource-state={state}
      description={recovery ? undefined : description}
      icon={compactState ? undefined : resolvedIcon}
      role={urgency === "assertive" ? "alert" : "status"}
      size={size}
      title={compactState ? (
        <span className="inline-flex items-center gap-2">
          {resolvedIcon}
          <span>{title}</span>
        </span>
      ) : title}
      tone={recovery ? tone : "default"}
      variant={variant}
      {...props}
    >
      {recovery ? (
        <RecoverySummary
          className={cn(
            "mt-1.5 w-full max-w-md",
            compactState ? "text-left" : "text-center",
          )}
          impact={<span data-resource-state-impact>{impact}</span>}
          nextStep={!primaryAction && !secondaryAction && nextStep
            ? <span data-resource-state-next-step>{nextStep}</span>
            : undefined}
        />
      ) : impact || nextStep ? (
        <div className={cn(
          "mt-3 w-full max-w-md space-y-1.5 break-words [overflow-wrap:anywhere]",
          getUiTypographyClassName({ role: "metadata", tone: "muted" }),
          compactState ? "text-left" : "text-center",
        )}>
          {impact ? (
            <p data-resource-state-impact>
              {impact}
            </p>
          ) : null}
          {nextStep ? (
            <p className="ui-type-tone-default ui-type-weight-medium" data-resource-state-next-step>
              {nextStep}
            </p>
          ) : null}
        </div>
      ) : null}
      {primaryAction || secondaryAction ? (
        <div className={cn(
          "flex w-full flex-col items-center justify-center gap-2 sm:w-auto sm:flex-row sm:flex-wrap",
          compactState
            ? "mt-2.5 flex-row flex-wrap justify-start sm:w-full sm:justify-start"
            : recovery
              ? "mt-2.5"
              : "mt-4",
        )}>
          {primaryAction ? (
            <ResourceStateAction action={primaryAction} primary />
          ) : null}
          {secondaryAction ? (
            <ResourceStateAction action={secondaryAction} />
          ) : null}
        </div>
      ) : null}
    </UiStateBlock>
  );
}

function ResourceStateAction({
  action,
  primary = false,
}: {
  action: UiResourceStateAction;
  primary?: boolean;
}) {
  return (
    <UiButton
      aria-busy={action.busy}
      className={cn(action.busy && "cursor-wait")}
      disabled={action.disabled || action.busy}
      onClick={action.onClick}
      size="sm"
      tone={action.tone === "danger"
        ? "danger"
        : primary
          ? "primary"
          : "default"}
      variant={primary ? "surface" : "text"}
    >
      {action.busy ? (
        <LoaderCircle
          aria-hidden
          className={getUiSpinnerClassName({ size: "sm" })}
        />
      ) : action.icon}
      {action.busy ? action.busyLabel ?? action.label : action.label}
    </UiButton>
  );
}
