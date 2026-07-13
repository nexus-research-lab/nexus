"use client";

import { type ReactNode } from "react";

import { UiBadge } from "@/shared/ui/display/badge";

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
    <UiBadge
      icon={icon}
      showDot={!icon}
      size={size === "compact" ? "xs" : "sm"}
      tone={tone}
    >
      {label}
    </UiBadge>
  );
}
