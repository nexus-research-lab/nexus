// INPUT: Owner-scoped Channel and Agent reads plus catalog-level committed callbacks.
// OUTPUT: Stale-safe directory snapshots refreshed by channel_control pushes, read recovery, and strict user feedback.
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
import { useChannelControlRefresh } from "../use-channel-control-refresh";
import type { ChannelFeedback } from "../channel-model";
import { filterChannels } from "./channel-catalog-model";

export function useChannelsController() {
  const { locale, t } = useI18n();
  const requestIdRef = useRef(0);
  const [channels, setChannels] = useState<ChannelConfigView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedType, setSelectedType] = useState<ImChannelType | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [readFailed, setReadFailed] = useState(false);
  const [feedback, setFeedback] = useState<ChannelFeedback | null>(null);

  const readSnapshot = useCallback(async (silent: boolean): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const [channelResult, agentResult] = await Promise.allSettled([
        listChannelsApi(locale),
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
      if (silent) {
        // 后台读取不改写命令反馈；失败由下一次读取或错误态承接。
      } else if (!complete) {
        setFeedback({
          impact: t("capability.channel_catalog_load_failed_impact"),
          nextStep: t("capability.channel_catalog_load_failed_next_step"),
          tone: "error",
          title: t("capability.channel_catalog_load_failed_title"),
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
        if (!silent) setFeedback({
          impact: t("capability.channel_catalog_load_failed_impact"),
          nextStep: t("capability.channel_catalog_load_failed_next_step"),
          tone: "error",
          title: t("capability.channel_catalog_load_failed_title"),
        });
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, [locale, t]);
  const refresh = useChannelControlRefresh(readSnapshot);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const visibleChannels = useMemo(() => filterChannels(
    channels,
    searchQuery,
  ), [channels, searchQuery]);
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
        title: t("capability.channel_saved_title"),
        message: t("capability.channel_saved_message", { channel: item.title }),
      });
    }
  }, [t]);

  const deleteChannel = useCallback(async (item: ChannelConfigView) => {
    const refreshed = await refresh();
    setSelectedType(null);
    setFeedback(refreshed
      ? {
          tone: "success",
          title: t("capability.channel_disconnected_title"),
          message: t("capability.channel_disconnected_message", {
            channel: item.title,
          }),
        }
      : {
          impact: t("state.committed_refresh_impact"),
          nextStep: t("state.committed_refresh_next_step"),
          tone: "error",
          title: t("capability.channel_disconnected_refresh_failed_title"),
        });
  }, [refresh, t]);

  const clearFeedback = useCallback(() => setFeedback(null), []);
  const closeChannel = useCallback(() => setSelectedType(null), []);
  const openChannel = useCallback((item: ChannelConfigView) => {
    setSelectedType(item.channel_type);
  }, []);

  return {
    agents,
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
    setSearchQuery,
    visibleChannels,
  };
}
