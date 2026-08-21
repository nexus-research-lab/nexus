/**
 * INPUT: 可见态与回到底部动作。
 * OUTPUT: 不随运行态闪动、只占自身热区的圆形回到底部入口。
 * POS: 主对话与 Thread 共用的唯一浮动滚动控件。
 */
import { ArrowDown } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "grid h-8 w-8 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) group-hover:border-(--surface-control-hover-border) group-hover:bg-(--surface-control-hover-background) group-hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  onClick: () => void;
  visible: boolean;
}

export function ScrollToLatestButton({
  onClick,
  visible,
}: ScrollToLatestButtonProps) {
  const { t } = useI18n();
  if (!visible) {
    return null;
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
