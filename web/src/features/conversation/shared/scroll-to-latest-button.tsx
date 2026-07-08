import { ArrowDown } from "lucide-react";

import { cn } from "@/lib/utils";

const FLOATING_ACTION_CHIP_CLASS_NAME =
  "absolute z-20 grid h-8 w-8 place-items-center rounded-full border border-[color:color-mix(in_srgb,var(--divider-subtle-color)_82%,var(--foreground)_10%)] bg-[color:color-mix(in_srgb,var(--background)_94%,var(--foreground)_6%)] text-(--text-default) shadow-[0_8px_22px_color-mix(in_srgb,var(--foreground)_10%,transparent)] transition-[transform,color,border-color,background] duration-(--motion-duration-fast) hover:-translate-y-[0.5px] hover:border-(--surface-interactive-active-border) hover:bg-[color:color-mix(in_srgb,var(--background)_90%,var(--foreground)_10%)] hover:text-(--text-strong)";

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
