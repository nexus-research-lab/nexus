import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

interface WorkspaceSurfaceToolbarActionProps {
  ariaLabel?: string;
  children: ReactNode;
  className?: string;
  disabled?: boolean;
  onClick?: () => void;
  title?: string;
  tone?: "default" | "primary";
}

export function WorkspaceSurfaceToolbarAction({
  ariaLabel,
  children,
  className,
  disabled = false,
  onClick,
  title,
  tone = "default",
}: WorkspaceSurfaceToolbarActionProps) {
  return (
    <button
      aria-label={ariaLabel}
      className={cn(
        "inline-flex items-center gap-1.5 text-[11px] font-semibold transition duration-(--motion-duration-fast) ease-out disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        tone === "default" && "text-(--text-default) hover:text-(--text-strong)",
        tone === "primary" && "text-(--primary) hover:text-[color:color-mix(in_srgb,var(--primary)_86%,var(--foreground)_14%)]",
        className,
      )}
      disabled={disabled}
      onClick={onClick}
      title={title}
      type="button"
    >
      {children}
    </button>
  );
}
