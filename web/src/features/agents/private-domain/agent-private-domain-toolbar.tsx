// INPUT: Private record title/count, loading state and refresh command.
// OUTPUT: Header metadata and the shared refresh control.
// POS: Private-domain header presentation; no resource reads or independent control styling.

import { RefreshCw } from "lucide-react";

import { cn } from "@/shared/ui/class-name";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
import { UiIconButton } from "@/shared/ui/button/button";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function PrivateDomainToolbar({
  count,
  isLoading: isLoading,
  onRefresh: onRefresh,
  refreshLabel,
  title,
}: {
  count: number;
  isLoading: boolean;
  onRefresh: () => void;
  refreshLabel: string;
  title: string;
}) {
  return (
    <div className="flex min-h-[48px] items-center justify-between gap-3 px-3">
      <div className="flex min-w-0 items-baseline gap-1.5">
        <span className={cn("truncate", getUiTypographyClassName({ role: "supporting", tone: "strong", weight: "semibold" }))}>{title}</span>
        <span className={cn("tabular-nums", getUiTypographyClassName({ role: "caption", tone: "soft" }))}>{count}</span>
      </div>
      <UiIconButton
        aria-label={refreshLabel}
        disabled={isLoading}
        onClick={onRefresh}
        size="xs"
        title={refreshLabel}
        variant="ghost"
      >
        <RefreshCw
          className={isLoading
            ? getUiSpinnerClassName({ size: "sm", tone: "muted" })
            : "h-3.5 w-3.5"}
        />
      </UiIconButton>
    </div>
  );
}
