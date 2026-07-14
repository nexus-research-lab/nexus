"use client";

import {
  Plus,
  RefreshCw,
  SearchX,
  ShieldCheck,
} from "lucide-react";

import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { cn } from "@/shared/ui/class-name";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WORKSPACE_DETAIL_PAGE_CLASS_NAME } from "@/shared/ui/layout/workspace-detail-layout";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { CreatePairingDialog } from "./pairings/pairing-create-dialog";
import { PairingFilterBar } from "./pairings/pairing-filter-bar";
import { PairingList } from "./pairings/pairing-list";
import { usePairingsController } from "./pairings/use-pairings-controller";

export function PairingsDirectory() {
  const { t } = useI18n();
  const controller = usePairingsController();
  const feedbackItem: FeedbackBannerProps | null = controller.feedback
    ? {
        message: controller.feedback.message,
        onDismiss: controller.clearFeedback,
        title: controller.feedback.title,
        tone: controller.feedback.tone,
      }
    : null;

  return (
    <>
      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={t("capability.pairings_badge", {
              count: controller.items.length,
            })}
            leading={<ShieldCheck className="h-4 w-4" />}
            subtitle={t("capability.pairings_subtitle")}
            title={t("capability.pairings")}
            trailing={(
              <>
                <WorkspaceSurfaceToolbarAction
                  disabled={controller.agents.length === 0 || controller.busy}
                  onClick={controller.openCreate}
                  title={controller.agents.length === 0
                    ? "需要先创建智能体"
                    : "新增 IM 配对"}
                  tone="primary"
                >
                  <Plus className="h-3.5 w-3.5" />
                  新增配对
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction
                  onClick={() => void controller.refresh()}
                >
                  <RefreshCw className="h-3.5 w-3.5" />
                  {t("capability.refresh")}
                </WorkspaceSurfaceToolbarAction>
              </>
            )}
          />
        )}
        stableGutter
      >
        <div
          className={cn(
            WORKSPACE_DETAIL_PAGE_CLASS_NAME,
            "max-w-[1280px] py-5",
          )}
        >
          {controller.loading && controller.items.length === 0 ? (
            <UiStateBlock
              description="正在同步外部 IM 用户与群聊的授权状态。"
              size="sm"
              title="加载配对..."
            />
          ) : controller.items.length === 0 ? (
            <PairingEmptyState
              busy={controller.busy}
              canCreate={controller.agents.length > 0}
              onCreate={controller.openCreate}
            />
          ) : (
            <>
              <PairingFilterBar
                agents={controller.agents}
                counts={controller.statusCounts}
                filters={controller.filters}
                onChange={controller.setFilter}
                searchPlaceholder={t("capability.pairings_search_placeholder")}
              />
              {controller.visibleItems.length === 0 ? (
                <PairingNoResults onClear={controller.clearFilters} />
              ) : (
                <PairingList
                  agents={controller.agents}
                  busy={controller.busy}
                  groups={controller.groups}
                  onCopySessionKey={controller.copySessionKey}
                  onDeletePairing={controller.requestDelete}
                  onUpdatePairing={controller.updatePairing}
                  pendingItems={controller.pendingItems}
                />
              )}
            </>
          )}
        </div>
      </WorkspaceSurfaceScaffold>

      {controller.createOpen ? (
        <CreatePairingDialog
          agents={controller.agents}
          onClose={controller.closeCreate}
          onCreated={controller.pairingCreated}
          onError={controller.reportCreateError}
        />
      ) : null}

      <FeedbackBannerViewport item={feedbackItem} />
      <ConfirmDialog
        confirmText="删除配对"
        isOpen={controller.deleteTarget !== null}
        message={controller.deleteTarget
          ? `确认删除 ${controller.deleteTarget.external_name || controller.deleteTarget.external_ref} 的配对吗？删除后该外部对象需要重新授权。`
          : ""}
        onCancel={() => controller.requestDelete(null)}
        onConfirm={controller.confirmDelete}
        title="删除配对"
        variant="danger"
      />
    </>
  );
}

function PairingEmptyState({
  busy,
  canCreate,
  onCreate,
}: {
  busy: boolean;
  canCreate: boolean;
  onCreate: () => void;
}) {
  return (
    <div className="flex min-h-[360px] flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
      <ShieldCheck className="h-8 w-8 text-(--icon-default)" />
      <h2 className="mt-4 text-[17px] font-semibold text-(--text-strong)">
        还没有配对
      </h2>
      <p className="mt-1 max-w-[460px] text-[13px] leading-6 text-(--text-muted)">
        外部 IM 用户或群首次发消息后会在这里等待授权，也可以手动新增配对。
      </p>
      <UiButton
        className="mt-5"
        disabled={!canCreate || busy}
        onClick={onCreate}
        title={canCreate ? "新增 IM 配对" : "需要先创建智能体"}
        tone="primary"
        type="button"
        variant="solid"
      >
        <Plus className="h-3.5 w-3.5" />
        {canCreate ? "新增配对" : "需要先创建智能体"}
      </UiButton>
    </div>
  );
}

function PairingNoResults({ onClear }: { onClear: () => void }) {
  return (
    <div className="flex min-h-[280px] flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
      <SearchX className="h-7 w-7 text-(--icon-muted)" />
      <h2 className="mt-3 text-[15px] font-semibold text-(--text-strong)">
        没有符合条件的配对
      </h2>
      <p className="mt-1 text-[13px] text-(--text-muted)">
        调整筛选条件，或清除筛选查看全部配对。
      </p>
      <UiButton className="mt-4" onClick={onClear} size="sm" type="button">
        清除筛选
      </UiButton>
    </div>
  );
}
