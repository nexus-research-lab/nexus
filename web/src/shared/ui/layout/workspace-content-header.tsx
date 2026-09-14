/**
 * INPUT: 管理页标题、用途说明、可选动作与二级页导航内容。
 * OUTPUT: 与 Workspace 内容轴一致、允许长内容自然增高的标准页头。
 * POS: 全站管理型内容页 Header owner；不解释具体业务动作或页面状态。
 */
"use client";

import type { ReactNode } from "react";

import { APP_NARROW_VIEWPORT_HIDDEN_CLASS_NAME } from "@/lib/layout/home-layout";
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

/** 管理页只保留一层正文标题，常规内容与动作对齐，长内容允许增高而不越过分隔线。 */
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
        "workspace-content-header mb-4 shrink-0 border-b border-(--divider-subtle-color) pb-4 sm:min-h-[var(--workspace-header-height,60px)] sm:pb-0",
        className,
      )}
      data-desktop-window-drag-region
      data-tour-anchor={headerAnchor}
    >
      <div className="workspace-content-header-inner flex min-h-[52px] flex-col gap-3 sm:min-h-[var(--workspace-header-height,60px)] sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
        <div className="min-w-0 flex-1 sm:basis-60">
          <h1 className={cn("[overflow-wrap:anywhere]", getUiTypographyClassName({ role: "pageTitle", tone: "strong" }))}>
            {title}
          </h1>
          {description ? (
            <p className={cn(
              "mt-0.5 max-w-[640px] [overflow-wrap:anywhere]",
              getUiTypographyClassName({ role: "metadata", tone: "muted" }),
            )}>
              {description}
            </p>
          ) : null}
        </div>
        {actions ? (
          <div className="flex min-h-8 min-w-0 max-w-full flex-wrap items-center gap-2 sm:ml-auto sm:justify-end">
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
        "workspace-content-header h-[var(--workspace-header-height,60px)] shrink-0",
        APP_NARROW_VIEWPORT_HIDDEN_CLASS_NAME,
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
