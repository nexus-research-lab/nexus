/**
 * INPUT: Loop 目录的加载、错误、空态或筛选结果。
 * OUTPUT: 单一状态消息或无卡片嵌套的 Loop 列表。
 * POS: Loop picker 的内容投影，不维护选择器生命周期。
 */
import type { ReactNode } from "react";
import { RotateCcw } from "lucide-react";

import type { ResourceFailure } from "@/lib/error-message";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import type { LoopCatalogItem } from "@/types/capability/loop";

import {
  type LoopPickerContentKind,
  projectLoopPickerContentKind,
} from "./loop-picker-model";
import { LoopPickerItem } from "./loop-picker-item";

export function LoopPickerContent({
  busySlug,
  error,
  hasCatalogItems,
  hasSnapshot,
  isLoading,
  loops,
  onClearFilters,
  onRetry,
  onSelect,
}: {
  busySlug: string | null;
  error: ResourceFailure | null;
  hasCatalogItems: boolean;
  hasSnapshot: boolean;
  isLoading: boolean;
  loops: LoopCatalogItem[];
  onClearFilters: () => void;
  onRetry: () => void;
  onSelect: (loop: LoopCatalogItem) => void | Promise<void>;
}) {
  const { t } = useI18n();
  const kind = projectLoopPickerContentKind({
    accessBlocked: Boolean(error?.access),
    error,
    hasSnapshot,
    isLoading,
    loopCount: loops.length,
  });
  const content: Record<LoopPickerContentKind, ReactNode> = {
    empty: (
      <UiResourceState
        className="min-h-48"
        impact={hasCatalogItems ? t("state.filter_impact") : undefined}
        primaryAction={hasCatalogItems ? {
          label: t("state.clear_filters"),
          onClick: onClearFilters,
        } : {
          icon: <RotateCcw className="h-3.5 w-3.5" />,
          label: t("state.retry"),
          onClick: onRetry,
        }}
        size="sm"
        state="empty"
        title={t("composer.loop_picker_empty")}
        variant="plain"
      />
    ),
    error: error ? (
      <UiResourceState
        className="min-h-48"
        impact={t(error.access
          ? "state.access_failure_impact"
          : "state.read_failure_impact")}
        primaryAction={{
          icon: <RotateCcw className="h-3.5 w-3.5" />,
          label: t("state.retry"),
          onClick: onRetry,
        }}
        size="sm"
        state="error"
        title={t(error.access
          ? "state.permission_title"
          : "composer.loop_picker_failed")}
        variant="plain"
      />
    ) : null,
    list: (
      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto rounded-[10px] border border-(--divider-subtle-color)">
        <div className="divide-y divide-(--divider-subtle-color)">
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
    loading: (
      <UiResourceState
        className="min-h-48"
        size="sm"
        state="loading"
        title={t("composer.loop_picker_loading")}
        variant="plain"
      />
    ),
  };
  const staleFailure = error && hasSnapshot && !error.access;
  return (
    <>
      {staleFailure ? (
        <UiResourceState
          className="min-h-0 py-3"
          impact={t("state.stale_snapshot_impact")}
          primaryAction={{
            icon: <RotateCcw className="h-3.5 w-3.5" />,
            label: t("state.retry"),
            onClick: onRetry,
          }}
          role="status"
          size="sm"
          state="error"
          title={t("composer.loop_picker_failed")}
          variant="plain"
        />
      ) : null}
      {content[kind]}
    </>
  );
}
