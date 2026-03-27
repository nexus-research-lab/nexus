"use client";

import { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceIconToggleButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  icon: ReactNode;
  is_active?: boolean;
  class_name?: string;
}

export function WorkspaceIconToggleButton({
  icon,
  is_active = false,
  className,
  class_name,
  type = "button",
  ...props
}: WorkspaceIconToggleButtonProps) {
  return (
    <button
      className={cn(
        "inline-flex h-10 w-10 items-center justify-center rounded-2xl border transition",
        is_active
          ? "workspace-card-strong text-slate-950"
          : "workspace-chip text-slate-700/60 hover:text-slate-950",
        className,
        class_name,
      )}
      type={type}
      {...props}
    >
      {icon}
    </button>
  );
}
