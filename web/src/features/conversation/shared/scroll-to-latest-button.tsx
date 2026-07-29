/**
 * INPUT: 输出加载态、可见态、Room 未读方向/数量与定位动作。
 * OUTPUT: 零布局占位的圆形回到底部入口，或只占自身热区的未读定位胶囊。
 * POS: 主对话与 Thread 共用的回到底部视觉控件。
 */
import { ArrowDown, ArrowUp } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "grid h-8 w-8 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) group-hover:border-(--surface-control-hover-border) group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  direction?: "above" | "below" | null;
  isLoading: boolean;
  onClick: () => void;
  unreadCount?: number;
  visible: boolean;
}

export function ScrollToLatestButton({
  direction = null,
  isLoading,
  onClick,
  unreadCount = 0,
  visible,
}: ScrollToLatestButtonProps) {
  if (!visible) {
    return null;
  }
  if (unreadCount > 0 && direction) {
    const DirectionIcon = direction === "above" ? ArrowUp : ArrowDown;
    const label = `${formatUnreadCount(unreadCount)} 条新消息`;
    return (
      <button
        type="button"
        aria-label={`${label}，定位到第一条`}
        onClick={onClick}
        className="group pointer-events-auto flex h-11 items-center justify-center"
        data-room-unread-jump
      >
        <span className="inline-flex h-8 items-center gap-1.5 rounded-full border border-[color:color-mix(in_srgb,var(--brand-action)_22%,var(--surface-control-border))] bg-(--surface-control-background) px-3 text-xs font-medium text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) group-hover:border-[color:color-mix(in_srgb,var(--brand-action)_34%,var(--surface-control-hover-border))] group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong)">
          <DirectionIcon
            aria-hidden="true"
            className="h-3.5 w-3.5 shrink-0 text-(--brand-action)"
          />
          <span>{label}</span>
        </span>
      </button>
    );
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

function formatUnreadCount(count: number): string {
  return count > 99 ? "99+" : String(count);
}
