// INPUT: 可用桌面版本、原生更新桥接和当前启动状态。
// OUTPUT: 共享圆形 IconButton 呈现的更新提示与忙碌状态。
// POS: 桌面侧栏更新动作；不负责检查版本或管理原生更新生命周期。

import { useState } from "react";
import { Download, LoaderCircle } from "lucide-react";

import { startDesktopUpdate } from "@/lib/desktop-bridge";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

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
    <UiIconButton
      aria-busy={starting}
      aria-label={label}
      className={cn("sidebar-update-indicator relative", className)}
      disabled={starting}
      onClick={() => void startUpdate()}
      shape="round"
      size="md"
      tooltip={label}
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
    </UiIconButton>
  );
}
