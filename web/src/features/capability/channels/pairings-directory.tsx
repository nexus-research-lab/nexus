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
import { ConfirmDialog } from "@/shared/ui/dialog/decision/decision-dialog";
import {
  completeFeedbackBanner,
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiResourceState } from "@/shared/ui/display/resource-state";
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
    ? completeFeedbackBanner(
        {
          ...controller.feedback,
          onDismiss: controller.feedback.dismissible === false
            ? undefined
            : controller.clearFeedback,
        },
        {
          impact: t("feedback.unconfirmed_impact"),
        },
      )
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
          ) : controller.readFailed && controller.items.length === 0 ? (
            <UiResourceState
              impact={t("capability.channel_pairing_catalog_load_failed_impact")}
              primaryAction={{
                label: t("capability.channel_reconcile_action"),
                onClick: () => void controller.refresh(),
              }}
              state="error"
              title={t("capability.channel_pairing_catalog_load_failed_title")}
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
          blocked={controller.busy}
          failure={controller.createRecoveryFeedback
            ? completeFeedbackBanner(
                controller.createRecoveryFeedback,
                {
                  impact: t("feedback.unconfirmed_impact"),
                },
              )
            : null}
          onClose={controller.closeCreate}
          onCreate={controller.createPairing}
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
    <UiResourceState
      className="min-h-[260px] border-b border-(--divider-subtle-color)"
      description={t("capability.pairings_empty_description")}
      icon={<ShieldCheck className="h-6 w-6 text-(--icon-default)" />}
      primaryAction={{
        disabled: !canCreate || busy,
        icon: <Plus className="h-3.5 w-3.5" />,
        label: canCreate
          ? t("capability.pairings_create")
          : "需要先创建智能体",
        onClick: onCreate,
        tone: "primary",
      }}
      state="empty"
      title={t("capability.pairings_empty_title")}
    />
  );
}

function PairingNoResults({ onClear }: { onClear: () => void }) {
  const { t } = useI18n();

  return (
    <UiResourceState
      className="min-h-[180px] border-y border-(--divider-subtle-color)"
      primaryAction={{
        label: t("capability.clear_filters"),
        onClick: onClear,
      }}
      state="empty"
      title={t("capability.pairings_no_results_title")}
    />
  );
}
