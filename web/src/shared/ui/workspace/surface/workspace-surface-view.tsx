"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { WorkspaceSurfaceScaffold } from "./workspace-surface-scaffold";

interface WorkspaceSurfaceViewProps {
  eyebrow: string;
  title: string;
  titleTrailing?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  bodyScrollable?: boolean;
  showEyebrow?: boolean;
  /** 中文注释：这里只允许滚动区和内容宽度的布局调整，不再承担视觉覆写。 */
  bodyClassName?: string;
  contentClassName?: string;
  maxWidthClassName?: string;
}

export function WorkspaceSurfaceView({
  eyebrow,
  title,
  titleTrailing: titleTrailing,
  action,
  children,
  bodyScrollable: bodyScrollable = true,
  showEyebrow: showEyebrow = true,
  bodyClassName: bodyClassName,
  contentClassName: contentClassName,
  maxWidthClassName: maxWidthClassName = "max-w-[760px]",
}: WorkspaceSurfaceViewProps) {
  return (
    <WorkspaceSurfaceScaffold
      bodyClassName={cn("px-4 py-4 sm:px-5 xl:px-6", bodyClassName)}
      bodyScrollable={bodyScrollable}
      header={(
        <div className={cn("px-5 xl:px-6", showEyebrow ? "py-3" : "py-2.5")}>
          <div className={cn("mx-auto flex w-full items-center justify-between gap-3", maxWidthClassName)}>
            <div className="min-w-0 flex-1">
              {showEyebrow ? (
                <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
                  {eyebrow}
                </p>
              ) : null}
              <div className={cn("flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1", showEyebrow && "mt-1")}>
                <h2 className="truncate text-[17px] font-black tracking-[-0.045em] text-(--text-strong)">
                  {title}
                </h2>
                {titleTrailing ? (
                  <div className="min-w-0 shrink text-(--text-default)">
                    {titleTrailing}
                  </div>
                ) : null}
              </div>
            </div>
            {action}
          </div>
          <div className={cn("mx-auto w-full", maxWidthClassName, showEyebrow ? "mt-3" : "mt-2")}>
            <div className="h-px w-full rounded-full bg-(--divider-subtle-color)" />
          </div>
        </div>
      )}
      stableGutter
    >
      <div
        className={cn("mx-auto w-full", maxWidthClassName, contentClassName)}
      >
        {children}
      </div>
    </WorkspaceSurfaceScaffold>
  );
}
