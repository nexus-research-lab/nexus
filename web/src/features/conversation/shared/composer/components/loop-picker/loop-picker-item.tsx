/**
 * INPUT: 单个 Loop、当前启动状态与选择动作。
 * OUTPUT: 具可读动作/元信息、可换行标题、共享禁用语义的连续目录行。
 * POS: Loop picker 列表项，只组合 Loop 内容与启动命令。
 */
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiListRow } from "@/shared/ui/list/list-row";
import { getUiTypographyClassName } from "@/shared/ui/typography/typography-styles";
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
    <UiListRow
      aria-busy={busySlug === loop.slug || undefined}
      disabled={busySlug !== null}
      variant="flush"
      onClick={() => void onSelect(loop)}
      right={(
        <span className={getUiTypographyClassName({
          role: "supporting",
          tone: "default",
          weight: "medium",
        })}>
          {actionLabel}
        </span>
      )}
    >
      <span className="min-w-0 flex-1">
        <span className={`break-words [overflow-wrap:anywhere] ${getUiTypographyClassName({
          role: "sectionTitle",
          tone: "strong",
        })}`}>
          {loop.title}
        </span>
        <span className={`mt-0.5 block line-clamp-2 ${getUiTypographyClassName({
          role: "metadata",
          tone: "muted",
        })}`}>
          {loop.description}
        </span>
        <span className={`mt-1 block ${getUiTypographyClassName({
          role: "metadata",
          tone: "muted",
        })}`}>
          {loop.category} · {loop.trigger_type}
        </span>
      </span>
    </UiListRow>
  );
}
