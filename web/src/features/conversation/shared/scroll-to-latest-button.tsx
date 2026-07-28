/**
 * INPUT: 输出加载态、可见态与回到底部动作。
 * OUTPUT: 占用固定安全带、不会遮挡正文或跨压相邻分割线的回到底部入口。
 * POS: 主对话与 Thread 共用的回到底部视觉控件。
 */
import { ArrowDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "pointer-events-auto absolute left-1/2 top-1/2 grid h-8 w-8 -translate-x-1/2 -translate-y-1/2 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)";

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
  return (
    <div className="pointer-events-none relative z-20 h-10 shrink-0">
      {visible ? (
        <button
          type="button"
          aria-label="回到底部"
          onClick={onClick}
          className={FLOATING_ACTION_CHIP_CLASS_NAME}
          title="回到底部"
        >
          <ArrowDown
            aria-hidden="true"
            className={cn(
              "block h-4 w-4 shrink-0",
              isLoading ? "animate-bounce" : null,
            )}
          />
        </button>
      ) : null}
    </div>
  );
}
