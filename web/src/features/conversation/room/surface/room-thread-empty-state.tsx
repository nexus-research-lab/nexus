"use client";

import { Loader2 } from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";

export function RoomThreadEmptyState({ isLoading }: { isLoading: boolean }) {
  const { t } = useI18n();
  return (
    <div className="flex min-h-32 items-center justify-center gap-2 px-4 text-center text-sm leading-6 text-(--text-muted)">
      {isLoading ? <Loader2 className="h-4 w-4 animate-spin" /> : null}
      <span>
        {t(isLoading ? "room.thread_waiting" : "room.thread_empty")}
      </span>
    </div>
  );
}
