"use client";

import {
  type HTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { getUiListRowPresentation } from "./list-row-model";

interface UiListRowProps extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  actions?: ReactNode;
  active?: boolean;
  activeTone?: "default" | "sidebar";
  children?: ReactNode;
  className?: string;
  description?: ReactNode;
  inactiveTone?: "default" | "muted";
  leading?: ReactNode;
  meta?: ReactNode;
  onClick?: () => void;
  right?: ReactNode;
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
  inactiveTone = "default",
  leading,
  meta,
  onClick: onClick,
  right,
  subtitleTrailing: subtitleTrailing,
  title,
  ...props
}: UiListRowProps) {
  const presentation = getUiListRowPresentation({
    active,
    activeTone,
    className,
    inactiveTone,
    interactive: Boolean(onClick),
  });
  return (
    <div
      className={presentation.className}
      {...props}
      onClick={onClick}
      onKeyDown={(event) => handleListRowKeyDown(event, props.onKeyDown, onClick)}
      role={presentation.role}
      tabIndex={presentation.tabIndex}
    >
      {leading}
      {children ?? (
        <UiListRowDefaultContent
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

function UiListRowDefaultContent({
  description,
  meta,
  subtitleTrailing,
  title,
}: Pick<
  UiListRowProps,
  "description" | "meta" | "subtitleTrailing" | "title"
>) {
  return (
    <div className="min-w-0 flex-1">
      <div className="flex min-w-0 items-center gap-2">
        <span className="min-w-0 flex-1 truncate text-base font-semibold leading-5">{title}</span>
        {meta}
      </div>
      {description || subtitleTrailing ? (
        <div className="mt-0.5 flex min-w-0 items-center gap-2">
          {description ? (
            <div className="min-w-0 flex-1 truncate text-compact leading-[1.125rem] text-(--text-muted)">
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
