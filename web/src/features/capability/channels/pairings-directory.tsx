"use client";

import {
  Filter,
  Plus,
  RefreshCw,
  ShieldCheck,
  Users,
} from "lucide-react";

import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import type {
  ImChannelType,
  ImPairingStatus,
} from "@/lib/api/capability/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import {
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import { CreatePairingDialog } from "./pairings/pairing-create-dialog";
import { PairingList } from "./pairings/pairing-list";
import {
  CHANNEL_OPTIONS,
  STATUS_LABELS,
} from "./pairings/pairing-options";
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
        <CapabilityPageLayout
          description={t("capability.pairings_intro_description")}
          title={t("capability.pairings_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              onChange={(value) => controller.setFilter("query", value)}
              placeholder={t("capability.pairings_search_placeholder")}
              value={controller.filters.query}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.pairings_filter_channel_aria")}
              leading={<Filter className="h-3.5 w-3.5" />}
              onChange={(value) => controller.setFilter(
                "channel",
                value as ImChannelType | "",
              )}
              options={[
                { value: "", label: "全部渠道" },
                ...CHANNEL_OPTIONS,
              ]}
              value={controller.filters.channel}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.pairings_filter_status_aria")}
              onChange={(value) => controller.setFilter(
                "status",
                value as ImPairingStatus | "",
              )}
              options={[
                { value: "", label: "全部状态" },
                ...Object.entries(STATUS_LABELS).map(([value, label]) => ({
                  value,
                  label,
                })),
              ]}
              value={controller.filters.status}
            />
            <CapabilityFilterSelect
              ariaLabel="按处理智能体筛选"
              className="sm:w-[220px]"
              leading={<Users className="h-3.5 w-3.5" />}
              onChange={(value) => controller.setFilter("agentId", value)}
              options={[
                { value: "", label: "全部智能体" },
                ...controller.agents.map((agent) => ({
                  value: agent.agent_id,
                  label: agent.name,
                })),
              ]}
              value={controller.filters.agentId}
            />
            <div className="shrink-0 text-[12px] font-semibold text-(--text-muted) sm:ml-auto">
              {controller.visibleItems.length} 个配对 · {controller.activeCount} 个已授权 · {controller.pendingCount} 个待处理
            </div>
          </CapabilityFilterBar>

          {controller.loading ? (
            <UiStateBlock
              description="正在同步外部 IM 用户与群聊的授权状态。"
              size="sm"
              title="加载配对..."
            />
          ) : controller.visibleItems.length === 0 ? (
            <UiStateBlock
              description="外部 IM 用户或群首次发消息后，会在这里等待授权。"
              icon={<ShieldCheck className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title="暂无配对请求"
            />
          ) : (
            <PairingList
              agents={controller.agents}
              busy={controller.busy}
              groups={controller.groups}
              onCopySessionKey={controller.copySessionKey}
              onDeletePairing={controller.requestDelete}
              onUpdatePairing={controller.updatePairing}
            />
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
