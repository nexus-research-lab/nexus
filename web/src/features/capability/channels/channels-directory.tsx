"use client";

import {
  Loader2,
  MessageCircle,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";

import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import { useI18n } from "@/shared/i18n/i18n-context";
import {
  type FeedbackBannerProps,
} from "@/shared/ui/feedback/feedback-banner";
import { FeedbackBannerViewport } from "@/shared/ui/feedback/feedback-banner-viewport";
import { UiStateBlock } from "@/shared/ui/display/state-block";
import { WorkspaceSurfaceHeader } from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceToolbarAction } from "@/shared/ui/workspace/surface/workspace-surface-toolbar-action";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";

import {
  CHANNEL_FILTER_OPTIONS,
  type ChannelFilter,
} from "./catalog/channel-catalog-model";
import { ChannelCard } from "./catalog/channel-card";
import { useChannelsController } from "./catalog/use-channels-controller";
import { ChannelConnectDialog } from "./connection/channel-connect-dialog";

function ChannelLoadingGrid() {
  return (
    <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
      <Loader2 className="h-5 w-5 animate-spin" />
    </div>
  );
}

export function ChannelsDirectory() {
  const { t } = useI18n();
  const controller = useChannelsController();
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
            badge={t("capability.channels_badge", {
              count: controller.channels.length || 6,
            })}
            leading={<MessageCircle className="h-4 w-4" />}
            subtitle={t("capability.channels_subtitle")}
            title={t("capability.channels")}
            trailing={(
              <WorkspaceSurfaceToolbarAction
                onClick={() => void controller.refresh()}
              >
                <RefreshCw className="h-3.5 w-3.5" />
                {t("capability.refresh")}
              </WorkspaceSurfaceToolbarAction>
            )}
          />
        )}
        stableGutter
      >
        <CapabilityPageLayout
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
              label={t("capability.category_label")}
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

          {controller.loading ? (
            <ChannelLoadingGrid />
          ) : controller.visibleChannels.length === 0 ? (
            <UiStateBlock
              description={t("capability.channels_empty_description")}
              icon={<MessageCircle className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title={t("capability.channels_empty_title")}
            />
          ) : (
            <section>
              <CapabilitySectionHeader
                count={t("capability.result_count", {
                  count: controller.visibleChannels.length,
                })}
                title={t("capability.channels_section_title")}
              />
              <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
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
          onError={controller.reportError}
          onSaved={controller.saveChannel}
        />
      ) : null}

      <FeedbackBannerViewport item={feedbackItem} />
    </>
  );
}
