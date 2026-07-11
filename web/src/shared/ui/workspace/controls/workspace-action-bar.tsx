"use client";

import { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

const ACTION_CARD_CLASS_NAME =
  "rounded-[12px] border border-(--divider-subtle-color) px-4 py-4 text-left transition-[transform,background,border-color] duration-(--motion-duration-fast) hover:-translate-y-px hover:bg-(--surface-interactive-hover-background) hover:border-(--surface-interactive-hover-border)";

interface WorkspaceActionBarProps {
  children: ReactNode;
  variant?: "pills" | "cards";
}

interface WorkspaceActionCardProps {
  icon: ReactNode;
  title: string;
  description?: string;
  onClick: () => void;
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
  onClick: onClick,
}: WorkspaceActionCardProps) {
  return (
    <button
      className={ACTION_CARD_CLASS_NAME}
      onClick={onClick}
      type="button"
    >
      {icon}
      <p className="mt-3 text-sm font-semibold text-(--text-strong)">{title}</p>
      {description ? (
        <p className="mt-1 text-xs leading-5 text-(--text-soft)">
          {description}
        </p>
      ) : null}
    </button>
  );
}
