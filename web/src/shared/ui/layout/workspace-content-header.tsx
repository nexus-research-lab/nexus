"use client";

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface WorkspaceContentHeaderProps {
  actions?: ReactNode;
  className?: string;
  description?: ReactNode;
  headerAnchor?: string;
  title: ReactNode;
}

interface WorkspaceContentDetailHeaderProps {
  children: ReactNode;
  className?: string;
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
        "workspace-content-header mb-4 shrink-0 border-b border-(--divider-subtle-color) pb-4 sm:h-[var(--workspace-header-height,60px)] sm:pb-0",
        className,
      )}
      data-desktop-window-drag-region
      data-tour-anchor={headerAnchor}
    >
      <div className="workspace-content-header-inner flex min-h-[52px] flex-col gap-3 sm:h-full sm:min-h-0 sm:flex-row sm:items-center sm:justify-between">
        <div className="min-w-0 flex-1">
          <h1 className={getUiTypographyClassName({ role: "pageTitle", tone: "strong" })}>
            {title}
          </h1>
          {description ? (
            <p className="mt-0.5 max-w-[640px] text-compact leading-4 text-(--text-muted)">
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

/** 二级页导航占用与管理页标题相同的顶栏高度，统一对齐原生窗口控件中线。 */
export function WorkspaceContentDetailHeader({
  children,
  className,
}: WorkspaceContentDetailHeaderProps) {
  return (
    <header
      className={cn(
        "workspace-content-header hidden h-[var(--workspace-header-height,60px)] shrink-0 lg:block",
        className,
      )}
      data-desktop-window-drag-region
    >
      <div className="workspace-content-header-inner flex h-full min-w-0 items-center">
        {children}
      </div>
    </header>
  );
}
