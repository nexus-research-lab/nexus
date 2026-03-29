"use client";

import { ReactNode } from "react";

import {
  HOME_AGENT_INSPECTOR_WIDTH_CLASS,
  HOME_AGENT_INSPECTOR_WRAPPER_CLASS,
} from "@/lib/home-layout";
import { cn } from "@/lib/utils";

interface WorkspaceInspectorShellProps {
  children: ReactNode;
  class_name?: string;
}

export function WorkspaceInspectorShell({
  children,
  class_name,
}: WorkspaceInspectorShellProps) {
  return (
    <div
      className={cn(
        HOME_AGENT_INSPECTOR_WRAPPER_CLASS,
        "overflow-hidden rounded-r-[32px] rounded-l-[24px] border-l workspace-divider bg-white/8 backdrop-blur-md",
        class_name,
      )}
    >
      <aside className={cn("flex min-h-0 flex-col bg-transparent", HOME_AGENT_INSPECTOR_WIDTH_CLASS)}>
        <div className="flex-1 overflow-y-auto scrollbar-hide">
          {children}
        </div>
      </aside>
    </div>
  );
}
