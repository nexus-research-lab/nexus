import type { ReactNode } from "react";

import { useI18n } from "@/shared/i18n/i18n-context";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  type LoopPickerContentKind,
  projectLoopPickerContentKind,
} from "./loop-picker-model";
import { LoopPickerItem } from "./loop-picker-item";

export function LoopPickerContent({
  busySlug,
  error,
  isLoading,
  loops,
  onSelect,
}: {
  busySlug: string | null;
  error: string | null;
  isLoading: boolean;
  loops: LoopCatalogItem[];
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const kind = projectLoopPickerContentKind({
    error,
    isLoading,
    loopCount: loops.length,
  });
  const content: Record<LoopPickerContentKind, ReactNode> = {
    empty: <LoopPickerMessage message={t("composer.loop_picker_empty")} />,
    error: <LoopPickerMessage destructive message={error ?? ""} />,
    list: (
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto pr-1">
        <div className="grid grid-cols-1 gap-2">
          {loops.map((loop) => (
            <LoopPickerItem
              key={loop.slug}
              busySlug={busySlug}
              loop={loop}
              onSelect={onSelect}
            />
          ))}
        </div>
      </div>
    ),
    loading: <LoopPickerMessage message={t("composer.loop_picker_loading")} />,
  };
  return content[kind];
}

function LoopPickerMessage({
  destructive = false,
  message,
}: {
  destructive?: boolean;
  message: string;
}) {
  const tone = destructive ? "text-(--destructive)" : "text-(--text-muted)";
  return (
    <div className={`py-10 text-center text-[13px] ${tone}`}>
      {message}
    </div>
  );
}
