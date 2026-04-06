"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceCanvasShellProps {
  children: ReactNode;
  is_joined_with_inspector?: boolean;
}

export function WorkspaceCanvasShell({
  children,
  is_joined_with_inspector = false,
}: WorkspaceCanvasShellProps) {
  return (
    <section
      className={cn(
        "relative glass-surface flex min-h-0 min-w-0 flex-1 overflow-hidden before:pointer-events-none before:absolute before:inset-x-0 before:top-0 before:h-24 before:bg-[linear-gradient(180deg,rgba(255,255,255,0.3),rgba(255,255,255,0.07)_44%,transparent)] after:pointer-events-none after:absolute after:right-[-4%] after:top-[-8%] after:h-36 after:w-64 after:bg-[radial-gradient(circle,rgba(var(--primary-rgb),0.08),transparent_70%)] after:opacity-90",
        is_joined_with_inspector ? "rounded-l-[32px] rounded-r-[24px]" : "radius-shell-xl",
      )}
    >
      {children}
    </section>
  );
}
