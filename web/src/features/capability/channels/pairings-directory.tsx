/**
 * INPUT: owner 配对资源、筛选、创建更新删除命令。
 * OUTPUT: 展示授权目的、外部对象与处理智能体的频道配对目录。
 * POS: “能力 > 频道配对”的唯一页面入口。
 */
"use client";

import {
  Plus,
  RefreshCw,
  ShieldCheck,
} from "lucide-react";

import { CapabilityPageLayout } from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiButton } from "@/shared/ui/button/button";
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import type { FeedbackBannerProps } from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiStateBlock } from "@/shared/ui/display/state-block";
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
        stableGutter
      >
        <CapabilityPageLayout
          actions={(
            <div className="flex items-center gap-2">
              <WorkspaceSurfaceToolbarAction
                onClick={() => void controller.refresh()}
              >
                <RefreshCw className="h-3.5 w-3.5" />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
              <WorkspaceSurfaceToolbarAction
                disabled={controller.agents.length === 0 || controller.busy}
                onClick={controller.openCreate}
                title={controller.agents.length === 0
                  ? "需要先创建智能体"
                  : "新增 IM 配对"}
                tone="primary"
              >
                <Plus className="h-3.5 w-3.5" />
                {t("capability.pairings_create")}
              </WorkspaceSurfaceToolbarAction>
            </div>
          )}
          description={t("capability.pairings_intro_description")}
          title={t("capability.pairings_intro_title")}
        >
          {controller.loading && controller.items.length === 0 ? (
            <UiStateBlock
              size="sm"
              title={t("capability.pairings_loading_title")}
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
        </CapabilityPageLayout>
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
  const { t } = useI18n();

  return (
    <div className="flex min-h-[260px] flex-col items-center justify-center border-b border-(--divider-subtle-color) px-6 text-center">
      <ShieldCheck className="h-7 w-7 text-(--icon-default)" />
      <h2 className="mt-3 text-base font-medium text-(--text-strong)">
        {t("capability.pairings_empty_title")}
      </h2>
      <p className="mt-1 max-w-[460px] text-compact leading-5 text-(--text-muted)">
        {t("capability.pairings_empty_description")}
      </p>
      <UiButton
        className="mt-4"
        disabled={!canCreate || busy}
        onClick={onCreate}
        title={canCreate ? "新增 IM 配对" : "需要先创建智能体"}
        tone="primary"
        type="button"
        variant="solid"
      >
        <Plus className="h-3.5 w-3.5" />
        {canCreate ? t("capability.pairings_create") : "需要先创建智能体"}
      </UiButton>
    </div>
  );
}

function PairingNoResults({ onClear }: { onClear: () => void }) {
  const { t } = useI18n();

  return (
    <div className="flex min-h-[180px] flex-col items-center justify-center border-y border-(--divider-subtle-color) px-6 text-center">
      <h2 className="text-base font-semibold text-(--text-strong)">
        {t("capability.pairings_no_results_title")}
      </h2>
      <UiButton className="mt-4" onClick={onClear} size="sm" type="button">
        {t("capability.clear_filters")}
      </UiButton>
    </div>
  );
}
