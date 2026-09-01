// INPUT: Owner-scoped Channel and Agent reads plus catalog-level committed callbacks.
// OUTPUT: Stale-safe directory snapshots, read recovery, and strict user feedback.
// POS: Channel catalog controller; reads never erase the last reliable snapshot.
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import {
  listChannelsApi,
  type ChannelConfigView,
  type ImChannelType,
} from "@/lib/api/capability/channel-api";
import type { Agent } from "@/types/agent/agent";
import { useI18n } from "@/shared/i18n/i18n-context";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import type { ChannelFeedback } from "../channel-model";
import {
  filterChannels,
  type ChannelFilter,
} from "./channel-catalog-model";

export function useChannelsController() {
  const { t } = useI18n();
  const requestIdRef = useRef(0);
  const [channels, setChannels] = useState<ChannelConfigView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedType, setSelectedType] = useState<ImChannelType | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [loading, setLoading] = useState(true);
  const [readFailed, setReadFailed] = useState(false);
  const [feedback, setFeedback] = useState<ChannelFeedback | null>(null);

  const refresh = useCallback(async (): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const [channelResult, agentResult] = await Promise.allSettled([
        listChannelsApi(),
        getAgents(),
      ]);
      if (requestId !== requestIdRef.current) {
        return false;
      }
      if (channelResult.status === "fulfilled") {
        setChannels(channelResult.value);
      }
      if (agentResult.status === "fulfilled") {
        setAgents(agentResult.value);
      }
      const complete = channelResult.status === "fulfilled"
        && agentResult.status === "fulfilled";
      setReadFailed(channelResult.status === "rejected");
      if (!complete) {
        setFeedback({
          impact: t("capability.channel_catalog_load_failed_impact"),
          nextStep: t("capability.channel_catalog_load_failed_next_step"),
          tone: "error",
          title: t("capability.channel_catalog_load_failed_title"),
          message: t("capability.channel_catalog_load_failed_message"),
        });
      } else {
        setFeedback(null);
      }
      return complete;
    } catch {
      // Promise.allSettled should make this unreachable, but preserving the
      // current snapshot is safer than projecting an empty directory.
      if (requestId === requestIdRef.current) {
        setReadFailed(true);
        setFeedback({
          impact: t("capability.channel_catalog_load_failed_impact"),
          nextStep: t("capability.channel_catalog_load_failed_next_step"),
          tone: "error",
          title: t("capability.channel_catalog_load_failed_title"),
          message: t("capability.channel_catalog_load_failed_message"),
        });
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [t]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const visibleChannels = useMemo(() => filterChannels(
    channels,
    channelFilter,
    searchQuery,
  ), [channelFilter, channels, searchQuery]);
  const selectedChannel = useMemo(() => channels.find(
    (item) => item.channel_type === selectedType,
  ) ?? null, [channels, selectedType]);

  const saveChannel = useCallback((item: ChannelConfigView, announce = true) => {
    // 写响应比在途列表请求更新；先使旧请求失效，避免旧快照回写。
    requestIdRef.current += 1;
    setLoading(false);
    setChannels((current) => current.map((value) => (
      value.channel_type === item.channel_type ? item : value
    )));
    notifyCapabilitySummaryMutated({
      source: "channels",
      action: "save",
      channel_type: item.channel_type,
    });
    if (announce) {
      setFeedback({
        tone: "success",
        title: "连接成功",
        message: `${item.title} 已完成配置`,
      });
    }
  }, []);

  const deleteChannel = useCallback(async (item: ChannelConfigView) => {
    const refreshed = await refresh();
    setSelectedType(null);
    setFeedback(refreshed
      ? {
          tone: "success",
          title: "频道已断开",
          message: `${item.title} 已移除配置`,
        }
      : {
          impact: t("state.committed_refresh_impact"),
          nextStep: t("state.committed_refresh_next_step"),
          tone: "error",
          title: "频道已断开，刷新失败",
          message: "请手动刷新频道列表确认最新状态",
        });
  }, [refresh, t]);

  const clearFeedback = useCallback(() => setFeedback(null), []);
  const closeChannel = useCallback(() => setSelectedType(null), []);
  const openChannel = useCallback((item: ChannelConfigView) => {
    setSelectedType(item.channel_type);
  }, []);

  return {
    agents,
    channelFilter,
    channels,
    clearFeedback,
    closeChannel,
    deleteChannel,
    feedback,
    loading,
    openChannel,
    readFailed,
    refresh,
    saveChannel,
    searchQuery,
    selectedChannel,
    setChannelFilter,
    setSearchQuery,
    visibleChannels,
  };
}
