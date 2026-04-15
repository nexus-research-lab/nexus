"use client";

import { ReactNode } from "react";

interface WorkspaceStatusBadgeProps {
  label: string;
  tone?: "active" | "running" | "idle" | "success" | "default";
  size?: "default" | "compact";
  icon?: ReactNode;
}

export function WorkspaceStatusBadge({
  label,
  tone = "default",
  size = "default",
  icon,
}: WorkspaceStatusBadgeProps) {
  return (
    <span
      className="status-badge"
      data-size={size}
      data-tone={tone}
    >
      {icon ?? <span className="h-2 w-2 rounded-full bg-current" />}
      {label}
    </span>
  );
}
