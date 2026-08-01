"use client";

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

interface WorkspaceContentHeaderProps {
  actions?: ReactNode;
  className?: string;
  description?: ReactNode;
  headerAnchor?: string;
  title: ReactNode;
}

/** 管理页只保留一层正文标题，标题、说明与动作始终共享同一垂直基线。 */
export function WorkspaceContentHeader({
  actions,
  className,
  description,
  headerAnchor,
  title,
}: WorkspaceContentHeaderProps) {
  return (
    <header
      className={cn(
        "mb-4 shrink-0 border-b border-(--divider-subtle-color) pb-4",
        className,
      )}
      data-tour-anchor={headerAnchor}
    >
      <div className="flex min-h-[52px] flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0 flex-1">
          <h1 className="text-lg font-semibold tracking-[-0.02em] text-(--text-strong)">
            {title}
          </h1>
          {description ? (
            <p className="mt-1 max-w-[640px] text-compact leading-5 text-(--text-muted)">
              {description}
            </p>
          ) : null}
        </div>
        {actions ? (
          <div className="flex min-h-8 shrink-0 items-center sm:justify-end">
            {actions}
          </div>
        ) : null}
      </div>
    </header>
  );
}
