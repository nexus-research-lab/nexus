// INPUT: 页面内容与响应式留白。
// OUTPUT: 允许横纵收缩的工作区 Flex 页面框架。
// POS: 页面几何所有者，不创建独立滚动容器。
"use client";

import { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

interface WorkspacePageFrameProps {
  children: ReactNode;
  contentPaddingClassName?: string;
}

export function WorkspacePageFrame({
  children,
  contentPaddingClassName: contentPaddingClassName = "p-4 sm:p-5 xl:p-6",
}: WorkspacePageFrameProps) {
  return (
    <section
      className={cn(
        "flex min-h-0 min-w-0 flex-1 flex-col bg-transparent",
        contentPaddingClassName,
      )}
    >
      {children}
    </section>
  );
}
