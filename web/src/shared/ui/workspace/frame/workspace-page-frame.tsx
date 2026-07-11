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
