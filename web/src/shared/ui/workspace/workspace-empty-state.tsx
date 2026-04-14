"use client";

import { ReactNode } from "react";

import { WorkspaceActionBar } from "./workspace-action-bar";

const EMPTY_STATE_SHELL_CLASS_NAME =
  "surface-card mx-auto max-w-[40rem] rounded-[32px] px-6 py-7 text-center sm:px-8";
const EMPTY_STATE_ICON_CLASS_NAME =
  "chip-default mx-auto flex h-14 w-14 items-center justify-center rounded-[20px]";

interface WorkspaceEmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  actions?: ReactNode;
}

export function WorkspaceEmptyState({
  icon,
  title,
  description,
  actions,
}: WorkspaceEmptyStateProps) {
  return (
    <div className="relative flex min-h-0 flex-1 items-center justify-center px-4 py-6 sm:px-6">
      <section className={EMPTY_STATE_SHELL_CLASS_NAME}>
        <div className={EMPTY_STATE_ICON_CLASS_NAME}>
          {icon}
        </div>
        <h1 className="mt-5 text-[28px] font-black tracking-[-0.05em] text-(--text-strong)">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-7 text-(--text-muted)">
          {description}
        </p>
        {actions ? (
          <WorkspaceActionBar>
            {actions}
          </WorkspaceActionBar>
        ) : null}
      </section>
    </div>
  );
}
