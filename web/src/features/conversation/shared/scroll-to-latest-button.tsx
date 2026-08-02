/**
 * INPUT: 可见态、Room 未读方向/数量与定位动作。
 * OUTPUT: 不随运行态闪动的圆形回到底部入口，或只占自身热区的未读定位胶囊。
 * POS: 主对话与 Thread 共用的回到底部视觉控件。
 */
import { ArrowDown, ArrowUp } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "grid h-8 w-8 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) group-hover:border-(--surface-control-hover-border) group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  direction?: "above" | "below" | null;
  onClick: () => void;
  unreadCount?: number;
  visible: boolean;
}

export function ScrollToLatestButton({
  direction = null,
  onClick,
  unreadCount = 0,
  visible,
}: ScrollToLatestButtonProps) {
  const { t } = useI18n();
  if (!visible) {
    return null;
  }
  if (unreadCount > 0 && direction) {
    const DirectionIcon = direction === "above" ? ArrowUp : ArrowDown;
    const count = formatUnreadCount(unreadCount);
    const label = t(
      unreadCount === 1
        ? "room.unread_count_one"
        : "room.unread_count_other",
      { count },
    );
    return (
      <button
        type="button"
        aria-label={t("room.unread_jump_first", { label })}
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
      aria-label={t("room.scroll_to_latest")}
      onClick={onClick}
      className="group pointer-events-auto grid h-11 w-11 place-items-center justify-self-center"
      data-scroll-to-latest
    >
      <span className={FLOATING_ACTION_CHIP_CLASS_NAME}>
        <ArrowDown
          aria-hidden="true"
          className="block h-4 w-4 shrink-0"
        />
      </span>
    </button>
  );
}

function formatUnreadCount(count: number): string {
  return count > 99 ? "99+" : String(count);
}
