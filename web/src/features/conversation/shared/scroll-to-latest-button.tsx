/**
 * INPUT: 输出加载态、可见态与回到底部动作。
 * OUTPUT: 零布局占位、只在真实按钮热区接收指针的回到底部悬浮入口。
 * POS: 主对话与 Thread 共用的回到底部视觉控件。
 */
import { ArrowDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "grid h-8 w-8 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) group-hover:border-(--surface-control-hover-border) group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  isLoading: boolean;
  onClick: () => void;
  visible: boolean;
}

export function ScrollToLatestButton({
  isLoading,
  onClick,
  visible,
}: ScrollToLatestButtonProps) {
  if (!visible) {
    return null;
  }
  return (
    <button
      type="button"
      aria-label="回到底部"
      onClick={onClick}
      className="group pointer-events-auto grid h-11 w-11 place-items-center justify-self-center"
      data-scroll-to-latest
    >
      <span className={FLOATING_ACTION_CHIP_CLASS_NAME}>
        <ArrowDown
          aria-hidden="true"
          className={cn(
            "block h-4 w-4 shrink-0",
            isLoading ? "animate-bounce" : null,
          )}
        />
      </span>
    </button>
  );
}
