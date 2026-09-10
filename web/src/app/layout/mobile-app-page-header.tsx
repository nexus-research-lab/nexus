/**
 * INPUT: 窄窗二级页标题、返回动作、页面动作挂载引用与共享平台几何。
 * OUTPUT: 返回、标题和页面级动作共处一行并对齐宿主窗口控件的应用页头。
 * POS: 手机应用壳的二级导航；动作内容由当前业务页面通过 Portal 提供。
 */

import { ArrowLeft } from "lucide-react";
import type { Ref } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiIconButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import {
  MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
  MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
} from "@/shared/ui/layout/mobile-shell-header-layout";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";

interface MobileAppPageHeaderProps {
  actionsRef?: Ref<HTMLDivElement>;
  onBack: () => void;
  title: string;
}

export function MobileAppPageHeader({
  actionsRef,
  onBack,
  title,
}: MobileAppPageHeaderProps) {
  const { t } = useI18n();
  return (
    <header
      className="shell-region-header shrink-0 pt-[env(safe-area-inset-top)]"
      data-desktop-window-drag-region
    >
      <div
        className={cn(
          "flex items-center gap-2",
          MOBILE_SHELL_HEADER_HEIGHT_CLASS_NAME,
          MOBILE_SHELL_HEADER_GUTTER_CLASS_NAME,
        )}
        data-desktop-window-controls-leading
      >
        <UiIconButton
          aria-label={t("common.back")}
          className="shrink-0"
          onClick={onBack}
          shape="round"
          size="lg"
          variant="ghost"
        >
          <ArrowLeft className="h-4 w-4" />
        </UiIconButton>
        <h1 className={cn(
          "min-w-0 flex-1 truncate",
          getUiTypographyClassName({ role: "sectionTitle", tone: "strong" }),
        )}>
          {title}
        </h1>
        <div
          className="ml-auto flex shrink-0 items-center"
          ref={actionsRef}
        />
      </div>
    </header>
  );
}
