import { ArrowDown } from "lucide-react";

import { cn } from "@/lib/utils";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "absolute z-20 inline-flex h-10 w-10 items-center justify-center rounded-full border border-(--chip-default-border) bg-(--chip-default-background) text-(--text-default) transition-[transform,color,border-color,background] duration-(--motion-duration-fast) hover:-translate-y-[0.5px] hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)";

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
      ? (isMobileLayout ? "bottom-4 right-3" : "bottom-4 right-4")
      : (isMobileLayout ? "bottom-24 right-2" : "bottom-24 right-3 sm:bottom-30 sm:right-8");

  return (
    <button
      type="button"
      aria-label="回到底部"
      onClick={onClick}
      className={cn(FLOATING_ACTION_CHIP_CLASS_NAME, placementClassName)}
      title="回到底部"
    >
      <ArrowDown className={isLoading ? "h-4 w-4 animate-bounce" : "h-4 w-4"} />
    </button>
  );
}
