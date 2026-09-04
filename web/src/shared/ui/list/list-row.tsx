// INPUT: 列表内容、共享密度/状态语义、可选悬停说明/行级动作与原生 div 属性。
// OUTPUT: 静态内容行或具统一键盘行为的单一交互列表行。
// POS: ListRow DOM 原语；不拥有资源、选择真相或业务命令生命周期。

"use client";

import {
  type HTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

import {
  getUiListRowPresentation,
  type UiListRowDensity,
} from "./list-row-model";

export type { UiListRowDensity } from "./list-row-model";

interface UiListRowProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  active?: boolean;
  activeTone?: "default" | "sidebar";
  children?: ReactNode;
  className?: string;
  description?: ReactNode;
  density?: UiListRowDensity;
  disabled?: boolean;
  inactiveTone?: "default" | "muted";
  leading?: ReactNode;
  meta?: ReactNode;
  onClick?: () => void;
  right?: ReactNode;
  subtitleTrailing?: ReactNode;
  title?: ReactNode;
  tooltip?: string;
}

export interface UiListRowContentProps {
  description?: ReactNode;
  meta?: ReactNode;
  subtitleTrailing?: ReactNode;
  title?: ReactNode;
}

export function UiListRow({
  actions,
  active = false,
  activeTone = "default",
  children,
  className,
  description,
  density = "default",
  disabled = false,
  inactiveTone = "default",
  leading,
  meta,
  onClick: onClick,
  right,
  subtitleTrailing: subtitleTrailing,
  title,
  tooltip,
  ...props
}: UiListRowProps) {
  const presentation = getUiListRowPresentation({
    active,
    activeTone,
    className,
    density,
    disabled,
    inactiveTone,
    interactive: Boolean(onClick),
  });
  return (
    <div
      className={presentation.className}
      {...props}
      aria-disabled={disabled || undefined}
      onClick={disabled ? undefined : onClick}
      onKeyDown={(event) => handleListRowKeyDown(
        event,
        props.onKeyDown,
        disabled ? undefined : onClick,
      )}
      role={presentation.role}
      tabIndex={presentation.tabIndex}
      title={tooltip}
    >
      {leading}
      {children ?? (
        <UiListRowContent
          description={description}
          meta={meta}
          subtitleTrailing={subtitleTrailing}
          title={title}
        />
      )}
      {right}
      {actions}
    </div>
  );
}

function handleListRowKeyDown(
  event: KeyboardEvent<HTMLDivElement>,
  onKeyDown: UiListRowProps["onKeyDown"],
  onClick: UiListRowProps["onClick"],
): void {
  onKeyDown?.(event);
  if (
    !onClick
    || event.defaultPrevented
    || event.target !== event.currentTarget
  ) {
    return;
  }
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    onClick();
  }
}

export function UiListRowContent({
  description,
  meta,
  subtitleTrailing,
  title,
}: UiListRowContentProps) {
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-2">
        <span className={cn(
          "min-w-0 flex-1 truncate",
          getUiTypographyClassName({ role: "sectionTitle" }),
        )}>{title}</span>
        {meta}
      </div>
      {description || subtitleTrailing ? (
        <div className="mt-0.5 flex min-w-0 items-center gap-2">
          {description ? (
            <div className={cn(
              "min-w-0 flex-1 truncate",
              getUiTypographyClassName({ role: "metadata", tone: "muted" }),
            )}>
              {description}
            </div>
          ) : (
            <span className="min-w-0 flex-1" />
          )}
          {subtitleTrailing}
        </div>
      ) : null}
    </div>
  );
}
