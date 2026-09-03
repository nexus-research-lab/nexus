/**
 * INPUT: 单个 Loop、当前启动状态与选择动作。
 * OUTPUT: 一次点击即可选择的扁平目录行。
 * POS: Loop picker 列表项，不复制弹窗动作或状态说明。
 */
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
  return (
    <button
      className="flex w-full items-center gap-4 bg-(--surface-raised-background) px-4 py-3 text-left transition-colors hover:bg-(--surface-interactive-hover-background) disabled:cursor-wait disabled:opacity-60"
      disabled={busySlug !== null}
      onClick={() => void onSelect(loop)}
      type="button"
    >
      <span className="min-w-0 flex-1">
        <span className="block text-base font-semibold text-(--text-strong)">
          {loop.title}
        </span>
        <span className="mt-0.5 block line-clamp-2 text-compact leading-5 text-(--text-muted)">
          {loop.description}
        </span>
        <span className="mt-1.5 block text-xs text-(--text-soft)">
          {loop.category} · {loop.trigger_type}
        </span>
      </span>
      <span className="shrink-0 text-xs font-semibold text-(--brand-action)">
          {actionLabel}
      </span>
    </button>
  );
}
