"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspacePageFrameProps {
  children: ReactNode;
  content_padding_class_name?: string;
}

export function WorkspacePageFrame({
  children,
  content_padding_class_name = "p-4 sm:p-5 xl:p-6",
}: WorkspacePageFrameProps) {
  return (
    <section
      className={cn(
        "flex min-h-0 min-w-0 flex-1 flex-col bg-transparent",
        content_padding_class_name,
      )}
    >
      {children}
    </section>
  );
}
