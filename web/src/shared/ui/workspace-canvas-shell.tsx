"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceCanvasShellProps {
  children: ReactNode;
  class_name?: string;
  is_joined_with_inspector?: boolean;
}

export function WorkspaceCanvasShell({
  children,
  class_name,
  is_joined_with_inspector = false,
}: WorkspaceCanvasShellProps) {
  return (
    <section
      className={cn(
        "home-glass-panel-subtle flex min-h-0 min-w-0 flex-1 overflow-hidden",
        is_joined_with_inspector ? "rounded-l-[32px] rounded-r-[24px]" : "radius-shell-xl",
        class_name,
      )}
    >
      {children}
    </section>
  );
}
