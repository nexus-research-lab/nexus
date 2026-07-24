import { ArrowLeft } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

interface MobileAppPageHeaderProps {
  onBack: () => void;
  title: string;
}

export function MobileAppPageHeader({
  onBack,
  title,
}: MobileAppPageHeaderProps) {
  const { t } = useI18n();
  return (
    <header className="shell-region-header shrink-0 pt-[env(safe-area-inset-top)]">
      <div className="flex h-[52px] items-center gap-2 px-2 sm:px-3">
        <button
          aria-label={t("common.back")}
          className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-(--text-strong) transition hover:bg-(--interaction-hover-background)"
          onClick={onBack}
          type="button"
        >
          <ArrowLeft className="h-4 w-4" />
        </button>
        <h1 className="min-w-0 flex-1 truncate text-[14px] font-semibold text-(--text-strong)">
          {title}
        </h1>
      </div>
    </header>
  );
}
