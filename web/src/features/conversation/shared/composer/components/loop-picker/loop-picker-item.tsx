import { UiButton } from "@/shared/ui/button/button";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { LoopCatalogItem } from "@/types/capability/loop";

export function LoopPickerItem({
  busySlug,
  loop,
  onSelect,
}: {
  busySlug: string | null;
  loop: LoopCatalogItem;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const actionLabel = busySlug === loop.slug
    ? t("composer.loop_starting")
    : t("composer.use_loop");
  const select = () => void onSelect(loop);
  return (
    <div className="rounded-[8px] border border-(--divider-subtle-color) bg-(--surface-raised-background) p-3 transition-colors hover:bg-(--surface-interactive-hover-background)">
      <div className="flex items-start justify-between gap-3">
        <button
          className="min-w-0 flex-1 text-left"
          disabled={busySlug !== null}
          onClick={select}
          type="button"
        >
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
              {loop.category}
            </span>
            <span className="rounded-[6px] bg-(--surface-interactive-hover-background) px-2 py-0.5 text-[11px] text-(--text-soft)">
              {loop.trigger_type}
            </span>
          </div>
          <div className="mt-2 text-[14px] font-semibold text-(--text-strong)">
            {loop.title}
          </div>
          <p className="mt-1 line-clamp-2 text-[12px] leading-5 text-(--text-muted)">
            {loop.description}
          </p>
        </button>
        <UiButton
          className="mt-1 shrink-0"
          disabled={busySlug !== null}
          onClick={select}
          size="xs"
          tone="primary"
          variant="solid"
        >
          {actionLabel}
        </UiButton>
      </div>
    </div>
  );
}
