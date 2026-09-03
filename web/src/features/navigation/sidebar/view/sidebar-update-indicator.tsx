import { useState } from "react";
import { Download, LoaderCircle } from "lucide-react";

import { startDesktopUpdate } from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";
import { UiTooltip } from "@/shared/ui/overlay/tooltip";

export function SidebarUpdateIndicator({
  className,
  version,
}: {
  className?: string;
  version: string;
}) {
  const { t } = useI18n();
  const [starting, setStarting] = useState(false);
  const label = starting
    ? t("sidebar.update_starting")
    : t("sidebar.update_available", { version });

  const startUpdate = async () => {
    if (starting) return;
    setStarting(true);
    try {
      const result = await startDesktopUpdate();
      if (result.status === "disabled" || result.status === "unavailable") {
        throw new Error(`Desktop update is ${result.status}`);
      }
    } catch (error) {
      console.error("[DesktopUpdate] Failed to start native update:", error);
    } finally {
      setStarting(false);
    }
  };

  return (
    <UiTooltip label={label}>
      <button
        aria-label={label}
        aria-busy={starting}
        className={cn("sidebar-update-indicator relative", className)}
        disabled={starting}
        onClick={() => void startUpdate()}
        type="button"
      >
        {starting ? (
          <LoaderCircle
            className={getUiSpinnerClassName(
              { size: "md" },
              "h-[18px] w-[18px]",
            )}
          />
        ) : (
          <Download className="h-[18px] w-[18px]" />
        )}
        <span className="absolute right-0 top-0 h-2 w-2 rounded-full border-2 border-(--surface-shell-directory-background) bg-(--primary)" />
      </button>
    </UiTooltip>
  );
}
