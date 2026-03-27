"use client";

import { ReactNode } from "react";

import { WorkspaceActionBar } from "./workspace-action-bar";

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
      <section className="workspace-card max-w-xl rounded-[32px] px-6 py-6 text-center sm:px-8">
        <div className="workspace-chip mx-auto flex h-14 w-14 items-center justify-center rounded-[20px]">
          {icon}
        </div>
        <h1 className="mt-5 text-[28px] font-black tracking-[-0.05em] text-slate-950/90">
          {title}
        </h1>
        <p className="mt-3 text-sm leading-6 text-slate-700/60">
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
