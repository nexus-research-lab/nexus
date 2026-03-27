"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

interface WorkspaceStatusBadgeProps {
  label: string;
  tone?: "active" | "running" | "idle" | "success" | "default";
  class_name?: string;
  icon?: ReactNode;
}

export function WorkspaceStatusBadge({
  label,
  tone = "default",
  class_name,
  icon,
}: WorkspaceStatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-2 rounded-full px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.12em]",
        tone === "running" && "bg-emerald-100 text-emerald-700",
        tone === "active" && "bg-sky-100 text-sky-700",
        tone === "idle" && "bg-slate-200 text-slate-700",
        tone === "success" && "bg-emerald-100 text-emerald-700",
        tone === "default" && "workspace-chip text-slate-700/76",
        class_name,
      )}
    >
      {icon ?? <span className="h-2 w-2 rounded-full bg-current" />}
      {label}
    </span>
  );
}
