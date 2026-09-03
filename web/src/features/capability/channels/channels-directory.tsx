/**
 * INPUT: 频道目录、状态筛选、连接命令和配置弹窗状态。
 * OUTPUT: 呈现平台用途、连接状态与使用事实的频道目录。
 * POS: “能力 > 频道”的唯一页面入口。
 */
"use client";

import {
  MessageCircle,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";

import {
  CAPABILITY_DIRECTORY_GRID_CLASS_NAME,
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  completeFeedbackBanner,
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner-contract";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiResourceState } from "@/shared/ui/display/resource-state";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import {
  CHANNEL_FILTER_OPTIONS,
  type ChannelFilter,
} from "./catalog/channel-catalog-model";
import { ChannelCard } from "./catalog/channel-card";
import { useChannelsController } from "./catalog/use-channels-controller";
import { ChannelConnectDialog } from "./connection/channel-connect-dialog";

export function ChannelsDirectory() {
  const { t } = useI18n();
  const controller = useChannelsController();
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
            <WorkspaceSurfaceToolbarAction
              onClick={() => void controller.refresh()}
            >
              <RefreshCw className="h-3.5 w-3.5" />
              {t("capability.refresh")}
            </WorkspaceSurfaceToolbarAction>
          )}
          description={t("capability.channels_intro_description")}
          title={t("capability.channels_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              onChange={controller.setSearchQuery}
              placeholder={t("capability.channels_search_placeholder")}
              value={controller.searchQuery}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.channels_filter_aria")}
              label={t("capability.status_label")}
              leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
              onChange={(value) => controller.setChannelFilter(
                value as ChannelFilter,
              )}
              options={CHANNEL_FILTER_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={controller.channelFilter}
            />
          </CapabilityFilterBar>

          {controller.loading && controller.channels.length === 0 ? (
            <UiResourceState
              className="min-h-40"
              size="md"
              state="loading"
              title={t("capability.channels_loading")}
            />
          ) : controller.readFailed && controller.channels.length === 0 ? (
            <UiResourceState
              impact={t("capability.channel_catalog_load_failed_impact")}
              primaryAction={{
                label: t("capability.channel_reconcile_action"),
                onClick: () => void controller.refresh(),
              }}
              size="md"
              state="error"
              title={t("capability.channel_catalog_load_failed_title")}
            />
          ) : controller.visibleChannels.length === 0 ? (
            <UiStateBlock
              description={t("capability.channels_empty_description")}
              icon={<MessageCircle className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title={t("capability.channels_empty_title")}
            />
          ) : (
            <section>
              <div className={CAPABILITY_DIRECTORY_GRID_CLASS_NAME}>
                {controller.visibleChannels.map((item) => (
                  <ChannelCard
                    item={item}
                    key={item.channel_type}
                    onConfigure={controller.openChannel}
                  />
                ))}
              </div>
            </section>
          )}
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      {controller.selectedChannel ? (
        <ChannelConnectDialog
          agents={controller.agents}
          item={controller.selectedChannel}
          key={controller.selectedChannel.channel_type}
          onClose={controller.closeChannel}
          onDeleted={controller.deleteChannel}
          onSaved={controller.saveChannel}
        />
      ) : null}

      <FeedbackBannerViewport item={feedbackItem} />
    </>
  );
}
