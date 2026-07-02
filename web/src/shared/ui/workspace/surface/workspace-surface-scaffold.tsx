/**
 * =====================================================
 * @File   : workspace-surface-scaffold.tsx
 * @Date   : 2026-04-11 16:13
 * @Author : leemysw
 * 2026-04-11 16:13   Create
 * =====================================================
 */

"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceSurfaceScaffoldProps {
  header?: ReactNode;
  children: ReactNode;
  body_class_name?: string;
  body_scrollable?: boolean;
  stable_gutter?: boolean;
}

/** 中文注释：统一 room、dm 与目录页主内容区的“头部 + 主画布”骨架，避免页面各自维护一套。 */
export function WorkspaceSurfaceScaffold({
  header,
  children,
  body_class_name: bodyClassName,
  body_scrollable: bodyScrollable = false,
  stable_gutter: stableGutter = false,
}: WorkspaceSurfaceScaffoldProps) {
  return (
    <>
      {header}
      <div
        className={cn(
          bodyScrollable
            ? "soft-scrollbar min-h-0 min-w-0 flex-1 overflow-y-auto"
            : "min-h-0 min-w-0 flex-1 overflow-hidden",
          bodyScrollable && stableGutter && "scrollbar-stable-gutter",
          bodyClassName,
        )}
      >
        {children}
      </div>
    </>
  );
}
