// INPUT: 消息明细的展开态、前导内容、摘要内容、语义 tone 与原生按钮属性。
// OUTPUT: 轻微悬浮底色的消息明细按钮；保持图标对齐，保留焦点、语义状态色与旋转箭头。
// POS: Message 领域公共交互组件；不判断 Thought、Tool 或 Process 业务状态。

import type { ButtonHTMLAttributes, ReactNode } from "react";
import { ChevronRight } from "lucide-react";

import { UiButton, type UiButtonTone } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";

type MessageDetailToggleTone = "active" | "danger" | "default";

interface MessageDetailToggleProps extends Omit<
  ButtonHTMLAttributes<HTMLButtonElement>,
  "children" | "className" | "type"
> {
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  expanded: boolean;
  leading: ReactNode;
  tone?: MessageDetailToggleTone;
}

const BUTTON_TONE: Readonly<Record<MessageDetailToggleTone, UiButtonTone>> = {
  active: "primary",
  danger: "danger",
  default: "default",
};

export function MessageDetailToggle({
  children,
  className,
  contentClassName,
  expanded,
  leading,
  tone = "default",
  ...props
}: MessageDetailToggleProps) {
  return (
    <UiButton
      {...props}
      aria-expanded={expanded}
      className={cn(
        "min-h-7 w-full min-w-0 justify-start gap-1.5 border-0 px-0 py-0.5 text-left font-normal",
        "shadow-none active:bg-transparent aria-[expanded=true]:bg-transparent [&:not(:disabled):hover]:bg-(--surface-interactive-hover-background)",
        tone === "default" && "aria-[expanded=true]:text-(--text-muted) [&:not(:disabled):hover]:text-(--text-strong) [&:not(:disabled):hover_span]:text-(--text-strong) [&:not(:disabled):hover_svg]:text-(--icon-strong)",
        className,
      )}
      size="sm"
      tone={BUTTON_TONE[tone]}
      variant="text"
    >
      <span className="flex min-w-5 shrink-0 items-center justify-center">{leading}</span>
      <span className={cn("min-w-0 flex-1 truncate", contentClassName)}>
        {children}
      </span>
      <ChevronRight
        aria-hidden="true"
        className={cn(
          "h-3.5 w-3.5 shrink-0 transition-transform duration-(--motion-duration-fast)",
          expanded && "rotate-90",
        )}
      />
    </UiButton>
  );
}
