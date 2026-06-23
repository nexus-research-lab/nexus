import { Handshake, RefreshCw } from "lucide-react";

import { cn } from "@/lib/utils";

export function PrivateDomainToolbar({
  count,
  is_loading,
  on_refresh,
  title,
}: {
  count: number;
  is_loading: boolean;
  on_refresh: () => void;
  title: string;
}) {
  return (
    <div className="flex h-11 items-center justify-between gap-3 border-b border-(--divider-subtle-color) px-3.5">
      <div className="flex min-w-0 items-center gap-2">
        <span className="flex h-7 w-7 items-center justify-center rounded-full bg-[color:color-mix(in_srgb,var(--primary)_10%,transparent)] text-primary">
          <Handshake className="h-3.5 w-3.5" />
        </span>
        <span className="truncate text-[13px] font-bold text-(--text-strong)">{title}</span>
        <span className="text-[11px] font-semibold text-(--text-soft)">{count}</span>
      </div>
      <button
        aria-label="刷新联络"
        className="flex h-7 w-7 items-center justify-center rounded-full text-(--icon-default) transition hover:bg-(--surface-interactive-hover-background) hover:text-(--text-strong)"
        onClick={on_refresh}
        type="button"
      >
        <RefreshCw className={cn("h-3.5 w-3.5", is_loading && "animate-spin")} />
      </button>
    </div>
  );
}
