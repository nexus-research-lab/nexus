"use client";

import { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspacePillButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  children: ReactNode;
  variant?: "default" | "strong" | "success" | "danger";
  size?: "sm" | "md" | "icon";
  class_name?: string;
}

export function WorkspacePillButton({
  children,
  className,
  class_name,
  type = "button",
  variant = "default",
  size = "md",
  ...props
}: WorkspacePillButtonProps) {
  return (
    <button
      className={cn(
        "inline-flex items-center justify-center gap-2 rounded-full font-semibold transition-all duration-300 disabled:cursor-not-allowed disabled:opacity-60",
        size === "icon" && "h-10 w-10 px-0",
        size === "sm" && "px-3 py-1.5 text-[11px]",
        size === "md" && "px-4 py-2.5 text-sm",
        variant === "default" && "workspace-chip text-slate-900/82 hover:text-slate-950",
        variant === "strong" && "bg-sky-500 text-white hover:bg-sky-600",
        variant === "success" && "bg-emerald-400 text-white hover:bg-emerald-500",
        variant === "danger" && "border border-rose-300/26 bg-rose-50/72 text-rose-600 hover:bg-rose-100/80",
        className,
        class_name,
      )}
      type={type}
      {...props}
    >
      {children}
    </button>
  );
}
