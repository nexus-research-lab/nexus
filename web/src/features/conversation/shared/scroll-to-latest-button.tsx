import { ArrowDown } from "lucide-react";

import { cn } from "@/shared/ui/class-name";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "absolute z-20 grid h-8 w-8 place-items-center rounded-full border border-(--surface-control-border) bg-(--surface-control-background) text-(--text-default) shadow-(--surface-control-shadow) transition-[color,border-color,background,box-shadow] duration-(--motion-duration-fast) hover:border-(--surface-control-hover-border) hover:bg-(--surface-control-hover-background) hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  isLoading: boolean;
  isMobileLayout: boolean;
  onClick: () => void;
  placement?: "composer" | "panel";
}

export function ScrollToLatestButton({
  isLoading: isLoading,
  isMobileLayout: isMobileLayout,
  onClick: onClick,
  placement = "composer",
}: ScrollToLatestButtonProps) {
  const placementClassName =
    placement === "panel"
      ? "bottom-4 left-1/2 -translate-x-1/2"
      : (isMobileLayout ? "bottom-24 left-1/2 -translate-x-1/2" : "bottom-24 left-1/2 -translate-x-1/2 sm:bottom-30");

  return (
    <button
      type="button"
      aria-label="回到底部"
      onClick={onClick}
      className={cn(FLOATING_ACTION_CHIP_CLASS_NAME, placementClassName)}
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
  );
}
