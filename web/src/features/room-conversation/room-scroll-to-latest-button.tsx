import { ArrowDown } from "lucide-react";

interface RoomScrollToLatestButtonProps {
  isLoading: boolean;
  isMobileLayout: boolean;
  onClick: () => void;
}

export function RoomScrollToLatestButton({
  isLoading,
  isMobileLayout,
  onClick,
}: RoomScrollToLatestButtonProps) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={
        isMobileLayout
          ? "workspace-chip absolute bottom-24 right-2 z-20 inline-flex items-center gap-2 rounded-full px-3 py-2 text-sm font-semibold text-slate-900/80 transition hover:-translate-y-0.5 hover:text-slate-950"
          : "workspace-chip absolute bottom-24 right-3 z-20 inline-flex items-center gap-2 rounded-full px-3 py-2 text-sm font-semibold text-slate-900/80 transition hover:-translate-y-0.5 hover:text-slate-950 sm:bottom-30 sm:right-8 sm:px-4 sm:py-2.5"
      }
    >
      <ArrowDown className={isLoading ? "h-4 w-4 animate-bounce" : "h-4 w-4"} />
      {!isMobileLayout && <span>回到底部</span>}
    </button>
  );
}
