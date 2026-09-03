"use client";

import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { getUiSpinnerClassName } from "@/shared/ui/display/spinner-styles";

export function RoomThreadEmptyState({ isLoading }: { isLoading: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-32 items-center justify-center gap-2 px-4 text-center text-sm leading-6 text-(--text-muted)">
      {isLoading ? (
        <Loader2
          className={getUiSpinnerClassName({ size: "md", tone: "muted" })}
        />
      ) : null}
      <span>
        {t(isLoading ? "room.thread_waiting" : "room.thread_empty")}
      </span>
    </div>
  );
}
