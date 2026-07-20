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
  children?: ReactNode;
  className?: string;
  description?: ReactNode;
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
  children,
  className,
  description,
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
    className,
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
      {presentation.showActiveIndicator ? (
        <span className="absolute left-0 top-1/2 h-6 w-px -translate-y-1/2 bg-(--primary)" />
      ) : null}

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
  if (!onClick || event.defaultPrevented) {
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
        <span className="min-w-0 flex-1 truncate text-[14px] font-semibold">{title}</span>
        {meta}
      </div>
      {description || subtitleTrailing ? (
        <div className="mt-1 flex min-w-0 items-center gap-2">
          {description ? (
            <div className="min-w-0 flex-1 truncate text-[12px] leading-5 text-(--text-muted)">
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
