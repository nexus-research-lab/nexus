/**
 * INPUT: 移动端二级页标题、返回动作与可选页面动作挂载引用。
 * OUTPUT: 返回、标题和页面级动作共处一行的移动端应用页头。
 * POS: 手机应用壳的二级导航；动作内容由当前业务页面通过 Portal 提供。
 */

import { ArrowLeft } from "lucide-react";
import type { Ref } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";

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
        className="flex h-[52px] items-center gap-2 px-2 sm:px-3"
        data-desktop-window-controls-leading
      >
        <button
          aria-label={t("common.back")}
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-(--text-strong) transition hover:bg-(--interaction-hover-background)"
          onClick={onBack}
          type="button"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="min-w-0 flex-1 truncate text-base font-semibold text-(--text-strong)">
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
