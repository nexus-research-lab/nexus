import type { ButtonHTMLAttributes, ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

type MessageActionTone = "default" | "success" | "danger";

const ACTION_TONE_CLASS_MAP: Record<MessageActionTone, string> = {
  default: "hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)",
  success: "text-(--success) hover:bg-[color:color-mix(in_srgb,var(--success)_10%,transparent)] hover:text-(--success)",
  danger: "text-(--destructive) hover:bg-[color:color-mix(in_srgb,var(--destructive)_10%,transparent)] hover:text-(--destructive)",
};

export function MessageActionButton({
  children,
  className,
  tone = "default",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & {
  children: ReactNode;
  tone?: MessageActionTone;
}) {
  return (
    <button
      className={cn(
        "rounded-lg p-1 text-(--icon-default) transition-colors duration-(--motion-duration-fast) focus-visible:ring-2 focus-visible:ring-primary/50",
        ACTION_TONE_CLASS_MAP[tone],
        className,
      )}
      type="button"
      {...props}
    >
      {children}
    </button>
  );
}
