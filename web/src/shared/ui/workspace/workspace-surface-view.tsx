"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

import { WorkspaceSurfaceScaffold } from "./workspace-surface-scaffold";

interface WorkspaceSurfaceViewProps {
  eyebrow: string;
  title: string;
  action?: ReactNode;
  children: ReactNode;
  /** 中文注释：这里只允许滚动区和内容宽度的布局调整，不再承担视觉覆写。 */
  body_class_name?: string;
  content_class_name?: string;
  max_width_class_name?: string;
}

export function WorkspaceSurfaceView({
  eyebrow,
  title,
  action,
  children,
  body_class_name,
  content_class_name,
  max_width_class_name = "max-w-[760px]",
}: WorkspaceSurfaceViewProps) {
  return (
    <WorkspaceSurfaceScaffold
      body_class_name={cn("px-4 py-4 sm:px-5 xl:px-6", body_class_name)}
      body_scrollable
      header={(
        <div className="border-b divider-subtle px-5 py-3 xl:px-6">
          <div className={cn("mx-auto flex w-full items-center justify-between gap-3", max_width_class_name)}>
            <div className="min-w-0">
              <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-(--text-soft)">
                {eyebrow}
              </p>
              <h2 className="mt-1 truncate text-[17px] font-black tracking-[-0.045em] text-(--text-strong)">
                {title}
              </h2>
            </div>
            {action}
          </div>
        </div>
      )}
      stable_gutter
    >
      <div
        className={cn("mx-auto w-full", max_width_class_name, content_class_name)}
      >
        {children}
      </div>
    </WorkspaceSurfaceScaffold>
  );
}
