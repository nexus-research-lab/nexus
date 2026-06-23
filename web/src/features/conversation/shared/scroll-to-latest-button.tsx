import { ArrowDown } from "lucide-react";

import { cn } from "@/lib/utils";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "absolute z-20 inline-flex h-10 w-10 items-center justify-center rounded-full border border-(--chip-default-border) bg-(--chip-default-background) text-(--text-default) transition-[transform,color,border-color,background] duration-(--motion-duration-fast) hover:-translate-y-[0.5px] hover:border-(--surface-interactive-active-border) hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)";

interface ScrollToLatestButtonProps {
  is_loading: boolean;
  is_mobile_layout: boolean;
  on_click: () => void;
  placement?: "composer" | "panel";
}

export function ScrollToLatestButton({
  is_loading,
  is_mobile_layout,
  on_click,
  placement = "composer",
}: ScrollToLatestButtonProps) {
  const placement_class_name =
    placement === "panel"
      ? (is_mobile_layout ? "bottom-4 right-3" : "bottom-4 right-4")
      : (is_mobile_layout ? "bottom-24 right-2" : "bottom-24 right-3 sm:bottom-30 sm:right-8");

  return (
    <button
      type="button"
      aria-label="回到底部"
      onClick={on_click}
      className={cn(FLOATING_ACTION_CHIP_CLASS_NAME, placement_class_name)}
      title="回到底部"
    >
      <ArrowDown className={is_loading ? "h-4 w-4 animate-bounce" : "h-4 w-4"} />
    </button>
  );
}
