// INPUT: 消息明细的展开态、前导内容、摘要内容、语义 tone 与原生按钮属性。
// OUTPUT: 常规字重、展开不加深的消息明细按钮；保留焦点、语义状态色与旋转箭头。
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
        "w-full min-w-0 justify-start text-left font-normal",
        tone === "default" && "aria-[expanded=true]:text-(--text-muted)",
        className,
      )}
      size="sm"
      tone={BUTTON_TONE[tone]}
      variant="text"
    >
      {leading}
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
