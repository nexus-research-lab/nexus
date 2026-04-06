"use client";

import { ReactNode } from "react";

import { cn } from "@/lib/utils";

const ACTION_CARD_CLASS_NAME =
  "surface-card rounded-[24px] px-4 py-4 text-left transition-[transform,background,border-color] duration-150 hover:-translate-y-px hover:bg-[var(--surface-interactive-hover-background)] hover:border-[var(--surface-interactive-hover-border)]";

interface WorkspaceActionBarProps {
  children: ReactNode;
  variant?: "pills" | "cards";
}

interface WorkspaceActionCardProps {
  icon: ReactNode;
  title: string;
  description?: string;
  on_click: () => void;
}

export function WorkspaceActionBar({
  children,
  variant = "pills",
}: WorkspaceActionBarProps) {
  return (
    <div
      className={cn(
        variant === "pills"
          ? "mt-6 flex flex-wrap items-center justify-center gap-3"
          : "mt-6 grid gap-3 md:grid-cols-3",
      )}
    >
      {children}
    </div>
  );
}

export function WorkspaceActionCard({
  icon,
  title,
  description,
  on_click,
}: WorkspaceActionCardProps) {
  return (
    <button
      className={ACTION_CARD_CLASS_NAME}
      onClick={on_click}
      type="button"
    >
      {icon}
      <p className="mt-3 text-sm font-semibold text-slate-950/86">{title}</p>
      {description ? (
        <p className="mt-1 text-xs leading-5 text-slate-700/58">
          {description}
        </p>
      ) : null}
    </button>
  );
}
