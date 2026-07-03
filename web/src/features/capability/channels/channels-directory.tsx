"use client";

import {
  Loader2,
  MessageCircle,
  RefreshCw,
  SlidersHorizontal,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { getAgents } from "@/lib/api/agent-manage-api";
import {
  ChannelConfigView,
  ImChannelType,
  listChannelsApi,
} from "@/lib/api/channel-api";
import { useI18n } from "@/shared/i18n/i18n-context";
import type { TranslationKey } from "@/shared/i18n/messages";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
  CapabilitySectionHeader,
} from "@/features/capability/shared/capability-page-layout";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";
import { ChannelCard } from "./channel-card";
import { ChannelConnectDialog } from "./channel-connect-dialog";
import { isChannelPlanned } from "./channel-model";

const CHANNEL_ORDER: ImChannelType[] = ["dingtalk", "wechat", "weixin-personal", "feishu", "telegram", "discord"];
type ChannelFilter = "all" | "connected" | "configured" | "unconfigured" | "planned";

const CHANNEL_FILTER_OPTIONS: ReadonlyArray<{ value: ChannelFilter; labelKey: TranslationKey }> = [
  { value: "all", labelKey: "capability.channels_filter_all" },
  { value: "connected", labelKey: "capability.channels_filter_connected" },
  { value: "configured", labelKey: "capability.channels_filter_configured" },
  { value: "unconfigured", labelKey: "capability.channels_filter_unconfigured" },
  { value: "planned", labelKey: "capability.channels_filter_planned" },
];

function ChannelLoadingGrid() {
  return (
    <div className="flex min-h-40 items-center justify-center text-sm text-(--text-muted)">
      <Loader2 className="h-5 w-5 animate-spin" />
    </div>
  );
}

export function ChannelsDirectory() {
  const { t } = useI18n();
  const [channels, setChannels] = useState<ChannelConfigView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selected, setSelected] = useState<ChannelConfigView | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [loading, setLoading] = useState(true);
  const [feedback, setFeedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const sortedChannels = useMemo(() => {
    return [...channels].sort((left, right) => {
      const leftIndex = CHANNEL_ORDER.indexOf(left.channel_type);
      const rightIndex = CHANNEL_ORDER.indexOf(right.channel_type);
      return (leftIndex < 0 ? CHANNEL_ORDER.length : leftIndex) - (rightIndex < 0 ? CHANNEL_ORDER.length : rightIndex);
    });
  }, [channels]);
  const visibleChannels = useMemo(() => {
    const query = searchQuery.trim().toLowerCase();
    return sortedChannels.filter((item) => {
      const matchesQuery = !query
        || item.title.toLowerCase().includes(query)
        || item.bot_label.toLowerCase().includes(query)
        || item.channel_type.toLowerCase().includes(query)
        || (item.agent_name ?? "").toLowerCase().includes(query);
      if (!matchesQuery) {
        return false;
      }
      if (channelFilter === "connected") {
        return item.connection_state === "connected";
      }
      if (channelFilter === "configured") {
        return item.configured && !isChannelPlanned(item);
      }
      if (channelFilter === "unconfigured") {
        return !item.configured && !isChannelPlanned(item);
      }
      if (channelFilter === "planned") {
        return isChannelPlanned(item);
      }
      return true;
    });
  }, [channelFilter, searchQuery, sortedChannels]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [nextChannels, nextAgents] = await Promise.all([listChannelsApi(), getAgents()]);
      setChannels(nextChannels);
      setAgents(nextAgents);
      return true;
    } catch (error) {
      setFeedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "频道加载失败" });
      return false;
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const handleChannelSaved = useCallback((item: ChannelConfigView, announce = true) => {
    setChannels((current) => current.map((value) => value.channel_type === item.channel_type ? item : value));
    notifyCapabilitySummaryMutated({ source: "channels", action: "save", channel_type: item.channel_type });
    if (announce) {
      setFeedback({ tone: "success", title: "连接成功", message: `${item.title} 已完成配置` });
    }
  }, []);

  const handleChannelDeleted = useCallback(async (item: ChannelConfigView) => {
    const refreshed = await refresh();
    setFeedback(
      refreshed
        ? { tone: "success", title: "频道已断开", message: `${item.title} 已移除配置` }
        : { tone: "error", title: "频道已断开，刷新失败", message: "请手动刷新频道列表确认最新状态" },
    );
  }, [refresh]);

  const feedbackItems: FeedbackBannerItem[] = feedback
    ? [{
        key: "channels-feedback",
        tone: feedback.tone,
        title: feedback.title,
        message: feedback.message,
        onDismiss: () => setFeedback(null),
      }]
    : [];

  return (
    <>
      <WorkspaceSurfaceScaffold
        bodyScrollable
        header={(
          <WorkspaceSurfaceHeader
            badge={t("capability.channels_badge", { count: channels.length || 6 })}
            density="compact"
            leading={<MessageCircle className="h-4 w-4" />}
            subtitle={t("capability.channels_subtitle")}
            title={t("capability.channels")}
            trailing={(
              <WorkspaceSurfaceToolbarAction onClick={() => void refresh()}>
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
              onChange={setSearchQuery}
              placeholder={t("capability.channels_search_placeholder")}
              value={searchQuery}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.channels_filter_aria")}
              label={t("capability.category_label")}
              leading={<SlidersHorizontal className="h-3.5 w-3.5" />}
              onChange={(value) => setChannelFilter(value as ChannelFilter)}
              options={CHANNEL_FILTER_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              }))}
              value={channelFilter}
            />
          </CapabilityFilterBar>

          {loading ? (
            <ChannelLoadingGrid />
          ) : visibleChannels.length === 0 ? (
            <UiStateBlock
              description={t("capability.channels_empty_description")}
              icon={<MessageCircle className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title={t("capability.channels_empty_title")}
            />
          ) : (
            <section>
              <CapabilitySectionHeader
                count={t("capability.result_count", { count: visibleChannels.length })}
                title={t("capability.channels_section_title")}
              />
              <div className="grid grid-cols-1 gap-x-12 gap-y-4 md:grid-cols-2">
                {visibleChannels.map((item) => (
                  <ChannelCard item={item} key={item.channel_type} onConfigure={setSelected} />
                ))}
              </div>
            </section>
          )}
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      {selected ? (
        <ChannelConnectDialog
          agents={agents}
          item={selected}
          onClose={() => setSelected(null)}
          onDeleted={handleChannelDeleted}
          onError={(message) => setFeedback({ tone: "error", title: "频道操作失败", message })}
          onSaved={handleChannelSaved}
        />
      ) : null}

      <FeedbackBannerStack items={feedbackItems} />
    </>
  );
}
