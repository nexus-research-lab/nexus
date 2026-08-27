// INPUT: 上层已经确定的加载、空、失败或完成展示内容与可选动作。
// OUTPUT: 统一回答“发生了什么、已有内容是否受影响、现在能做什么”的可访问展示面。
// POS: 纯展示组件；不判断 query、mutation、access、离线或重试语义。
"use client";

import type { HTMLAttributes, ReactNode } from "react";
import {
  CheckCircle2,
  CircleAlert,
  Inbox,
  LoaderCircle,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import type {
  UiStateBlockSize,
  UiStateBlockVariant,
} from "@/shared/ui/display/state-block-styles";

export type UiResourceStateKind =
  | "loading"
  | "empty"
  | "error"
  | "success";

export interface UiResourceStateAction {
  busy?: boolean;
  busyLabel?: ReactNode;
  disabled?: boolean;
  icon?: ReactNode;
  label: ReactNode;
  onClick: () => void;
}

interface UiResourceStateBaseProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  className?: string;
  description?: ReactNode;
  icon?: ReactNode;
  size?: UiStateBlockSize;
  title: ReactNode;
  variant?: UiStateBlockVariant;
}

interface UiResourceLoadingStateProps extends UiResourceStateBaseProps {
  impact?: ReactNode;
  nextStep?: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "loading";
}

interface UiResourceEmptyStateProps extends UiResourceStateBaseProps {
  impact?: ReactNode;
  nextStep: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "empty";
}

interface UiResourceFailureStateProps extends UiResourceStateBaseProps {
  impact: ReactNode;
  nextStep: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "error";
}

interface UiResourceSuccessStateProps extends UiResourceStateBaseProps {
  impact?: ReactNode;
  nextStep: ReactNode;
  primaryAction?: UiResourceStateAction;
  secondaryAction?: UiResourceStateAction;
  state: "success";
}

export type UiResourceStateProps =
  | UiResourceLoadingStateProps
  | UiResourceEmptyStateProps
  | UiResourceFailureStateProps
  | UiResourceSuccessStateProps;

const DEFAULT_STATE_ICONS: Record<UiResourceStateKind, ReactNode> = {
  empty: <Inbox className="h-5 w-5 text-(--icon-default)" />,
  error: <CircleAlert className="h-5 w-5 text-(--destructive)" />,
  loading: <LoaderCircle className="h-5 w-5 animate-spin text-(--icon-muted)" />,
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
  variant,
  ...props
}: UiResourceStateProps) {
  const { t } = useI18n();
  const failure = state === "error";
  const liveRole = failure ? "alert" : "status";

  return (
    <UiStateBlock
      aria-busy={state === "loading"}
      className={className}
      data-resource-state={state}
      description={description}
      icon={icon ?? DEFAULT_STATE_ICONS[state]}
      role={liveRole}
      size={size}
      title={title}
      tone={failure ? "danger" : "default"}
      variant={variant}
      {...props}
    >
      {impact || nextStep ? (
        <dl className="mt-4 w-full max-w-md space-y-2 rounded-[8px] border border-(--divider-subtle-color) bg-[color:color-mix(in_srgb,var(--surface-panel-background)_72%,transparent)] px-3 py-2.5 text-left text-xs leading-5 text-(--text-muted)">
          {impact ? (
            <div className="flex items-start gap-2" data-resource-state-impact>
              <dt className="shrink-0 font-semibold text-(--text-default)">
                {t("state.existing_data")}
              </dt>
              <dd className="min-w-0">{impact}</dd>
            </div>
          ) : null}
          {nextStep ? (
            <div className="flex items-start gap-2" data-resource-state-next-step>
              <dt className="shrink-0 font-semibold text-(--text-default)">
                {t("state.next_step")}
              </dt>
              <dd className="min-w-0">{nextStep}</dd>
            </div>
          ) : null}
        </dl>
      ) : null}
      {primaryAction || secondaryAction ? (
        <div className="mt-4 flex flex-wrap items-center justify-center gap-2">
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
      tone={primary ? "primary" : "default"}
      variant={primary ? "surface" : "text"}
    >
      {action.busy ? (
        <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
      ) : action.icon}
      {action.busy ? action.busyLabel ?? action.label : action.label}
    </UiButton>
  );
}
