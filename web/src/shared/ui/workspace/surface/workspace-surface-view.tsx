"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { WorkspaceSurfaceScaffold } from "./workspace-surface-scaffold";

interface WorkspaceSurfaceViewProps {
  eyebrow: string;
  title: string;
  title_trailing?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
  body_scrollable?: boolean;
  show_eyebrow?: boolean;
  /** 中文注释：这里只允许滚动区和内容宽度的布局调整，不再承担视觉覆写。 */
  body_class_name?: string;
  content_class_name?: string;
  max_width_class_name?: string;
}

export function WorkspaceSurfaceView({
  eyebrow,
  title,
  title_trailing: titleTrailing,
  action,
  children,
  body_scrollable: bodyScrollable = true,
  show_eyebrow: showEyebrow = true,
  body_class_name: bodyClassName,
  content_class_name: contentClassName,
  max_width_class_name: maxWidthClassName = "max-w-[760px]",
}: WorkspaceSurfaceViewProps) {
  return (
    <WorkspaceSurfaceScaffold
      body_class_name={cn("px-4 py-4 sm:px-5 xl:px-6", bodyClassName)}
      body_scrollable={bodyScrollable}
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
      stable_gutter
    >
      <div
        className={cn("mx-auto w-full", maxWidthClassName, contentClassName)}
      >
        {children}
      </div>
    </WorkspaceSurfaceScaffold>
  );
}
