/**
 * INPUT: Slash 指令名称与 Composer/消息显示位置。
 * OUTPUT: 不接管交互且单行文字不裁剪的轻量命令标签。
 * POS: Slash 指令在输入镜像与用户消息中的共享视觉原语。
 */
import type { ReactNode } from "react";

import { cn } from "@/shared/ui/class-name";

export function SlashCommandToken({
  children,
  variant = "message",
}: {
  children: ReactNode;
  variant?: "composer" | "message";
}) {
  return (
    <span
      className={cn(
        "inline-flex max-w-full items-center rounded-[5px] text-primary ring-1 ring-inset ring-primary/15",
        variant === "composer"
          ? "-mx-1 bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--input-shell-background))] px-1 font-normal leading-6"
          : "bg-[color:color-mix(in_srgb,var(--primary)_10%,var(--surface-message-user-background))] px-1.5 py-0.5 align-middle text-[0.9em] font-medium leading-tight",
      )}
      data-slash-command-token="true"
    >
      <span className="truncate">{children}</span>
    </span>
  );
}
