import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgents } from "@/lib/api/agent-manage-api";
import {
  listChannelsApi,
  type ChannelConfigView,
  type ImChannelType,
} from "@/lib/api/channel-api";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import type { ChannelFeedback } from "../channel-model";
import {
  filterChannels,
  type ChannelFilter,
} from "./channel-catalog-model";

export function useChannelsController() {
  const requestIdRef = useRef(0);
  const [channels, setChannels] = useState<ChannelConfigView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedType, setSelectedType] = useState<ImChannelType | null>(null);
  const [searchQuery, setSearchQuery] = useState("");
  const [channelFilter, setChannelFilter] = useState<ChannelFilter>("all");
  const [loading, setLoading] = useState(true);
  const [feedback, setFeedback] = useState<ChannelFeedback | null>(null);

  const refresh = useCallback(async (): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    setLoading(true);
    try {
      const [nextChannels, nextAgents] = await Promise.all([
        listChannelsApi(),
        getAgents(),
      ]);
      if (requestId !== requestIdRef.current) {
        return false;
      }
      setChannels(nextChannels);
      setAgents(nextAgents);
      return true;
    } catch (error) {
      if (requestId === requestIdRef.current) {
        setFeedback({
          tone: "error",
          title: "加载失败",
          message: error instanceof Error ? error.message : "频道加载失败",
        });
      }
      return false;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, []);

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
          tone: "error",
          title: "频道已断开，刷新失败",
          message: "请手动刷新频道列表确认最新状态",
        });
  }, [refresh]);

  const reportError = useCallback((message: string) => {
    setFeedback({ tone: "error", title: "频道操作失败", message });
  }, []);
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
    refresh,
    reportError,
    saveChannel,
    searchQuery,
    selectedChannel,
    setChannelFilter,
    setSearchQuery,
    visibleChannels,
  };
}
