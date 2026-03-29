"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspacePageFrameProps {
  children: ReactNode;
  body_class_name?: string;
  panel_class_name?: string;
  content_padding_class_name?: string;
  use_default_panel_style?: boolean;
}

export function WorkspacePageFrame({
  children,
  body_class_name,
  panel_class_name,
  content_padding_class_name = "p-4 sm:p-6",
  use_default_panel_style = true,
}: WorkspacePageFrameProps) {
  return (
    <div className={cn("relative flex min-h-0 flex-1 flex-col", body_class_name)}>
      <section
        className={cn(
          "relative flex min-h-0 flex-1 flex-col overflow-hidden",
          use_default_panel_style && "home-glass-panel-subtle radius-shell-xl",
          content_padding_class_name,
          panel_class_name,
        )}
      >
        {children}
      </section>
    </div>
  );
}
