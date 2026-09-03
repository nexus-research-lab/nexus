// INPUT: Workspace Header 的轻量动作、状态、tone 与原生点击命令。
// OUTPUT: 使用共享 caption 排版和状态色的无背景工具栏按钮。
// POS: Workspace Header 动作原语；不拥有业务权限或事务状态。

import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

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
        "inline-flex items-center gap-1.5 transition duration-(--motion-duration-fast) ease-out disabled:cursor-not-allowed disabled:opacity-(--disabled-opacity)",
        getUiTypographyClassName({
          role: "caption",
          tone: tone === "primary" ? "brand" : "default",
          weight: "semibold",
        }),
        tone === "default" && "hover:text-(--text-strong)",
        tone === "primary" && "hover:text-[color:color-mix(in_srgb,var(--brand-action)_86%,var(--foreground)_14%)]",
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
