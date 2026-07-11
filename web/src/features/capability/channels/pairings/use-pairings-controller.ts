import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { getAgents } from "@/lib/api/agent/agent-api";
import {
  deletePairingApi,
  listPairingsApi,
  updatePairingApi,
  type PairingView,
  type UpdatePairingPayload,
} from "@/lib/api/capability/channel-api";
import { getErrorMessage } from "@/lib/error-message";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../../capability-summary-events";
import type { ChannelFeedback } from "../channel-model";
import {
  countPairingStatus,
  filterPairings,
  groupPairings,
  pairingDisplayName,
  pairingSessionKey,
  type PairingFilters,
} from "./pairing-model";
import { usePairingCommand } from "./use-pairing-command";

const INITIAL_FILTERS: PairingFilters = {
  agentId: "",
  channel: "",
  query: "",
  status: "",
};

export function usePairingsController() {
  const requestIdRef = useRef(0);
  const { copy } = useCopyToClipboard();
  const [items, setItems] = useState<PairingView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [filters, setFilters] = useState<PairingFilters>(INITIAL_FILTERS);
  const [loading, setLoading] = useState(true);
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PairingView | null>(null);
  const [feedback, setFeedback] = useState<ChannelFeedback | null>(null);
  const { pendingAction, runCommand } = usePairingCommand();
  const filtersRef = useRef(filters);
  filtersRef.current = filters;

  const refresh = useCallback(async (reportError = true): Promise<boolean> => {
    const requestId = ++requestIdRef.current;
    const currentFilters = filtersRef.current;
    setLoading(true);
    try {
      const [nextItems, nextAgents] = await Promise.all([
        listPairingsApi({
          agent_id: currentFilters.agentId,
          channel_type: currentFilters.channel,
          status: currentFilters.status,
        }),
        getAgents(),
      ]);
      if (requestId === requestIdRef.current) {
        setItems(nextItems);
        setAgents(nextAgents);
      }
      return true;
    } catch (error) {
      if (requestId === requestIdRef.current && reportError) {
        setFeedback({
          tone: "error",
          title: "加载失败",
          message: getErrorMessage(error, "配对列表加载失败"),
        });
      }
      return requestId !== requestIdRef.current;
    } finally {
      if (requestId === requestIdRef.current) {
        setLoading(false);
      }
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [filters.agentId, filters.channel, filters.status, refresh]);

  const visibleItems = useMemo(
    () => filterPairings(items, filters.query),
    [filters.query, items],
  );
  const groups = useMemo(
    () => groupPairings(visibleItems, agents),
    [agents, visibleItems],
  );

  const updatePairing = useCallback(async (
    item: PairingView,
    next: UpdatePairingPayload,
  ) => {
    await runCommand({ kind: "update", pairingId: item.pairing_id }, async () => {
      try {
        const updated = await updatePairingApi(item.pairing_id, next);
        notifyCapabilitySummaryMutated({
          source: "pairings",
          action: "update",
          pairing_id: updated.pairing_id,
        });
        const refreshed = await refresh(false);
        setFeedback(refreshed
          ? {
              tone: "success",
              title: "配对已更新",
              message: `${pairingDisplayName(updated)} 已保存`,
            }
          : {
              tone: "error",
              title: "配对已更新，刷新失败",
              message: "请手动刷新配对列表确认最新状态",
            });
      } catch (error) {
        setFeedback({
          tone: "error",
          title: "更新失败",
          message: getErrorMessage(error, "配对更新失败"),
        });
      }
    });
  }, [refresh, runCommand]);

  const deletePairing = useCallback(async (item: PairingView) => {
    await runCommand({ kind: "delete", pairingId: item.pairing_id }, async () => {
      try {
        await deletePairingApi(item.pairing_id);
        notifyCapabilitySummaryMutated({
          source: "pairings",
          action: "delete",
          pairing_id: item.pairing_id,
        });
        const refreshed = await refresh(false);
        setFeedback(refreshed
          ? {
              tone: "success",
              title: "配对已删除",
              message: `${pairingDisplayName(item)} 已移除`,
            }
          : {
              tone: "error",
              title: "配对已删除，刷新失败",
              message: "请手动刷新配对列表确认最新状态",
            });
      } catch (error) {
        setFeedback({
          tone: "error",
          title: "删除失败",
          message: getErrorMessage(error, "配对删除失败"),
        });
      }
    });
  }, [refresh, runCommand]);

  const confirmDelete = useCallback(() => {
    const target = deleteTarget;
    setDeleteTarget(null);
    if (target) {
      void deletePairing(target);
    }
  }, [deletePairing, deleteTarget]);

  const pairingCreated = useCallback((item: PairingView) => {
    setFeedback({
      tone: "success",
      title: "配对已新增",
      message: `${pairingDisplayName(item)} 已创建`,
    });
    void refresh();
  }, [refresh]);

  const copySessionKey = useCallback(async (item: PairingView) => {
    const key = pairingSessionKey(item);
    const copied = await copy(key);
    setFeedback(copied
      ? { tone: "success", title: "Session 已复制", message: key }
      : { tone: "error", title: "复制失败", message: "无法复制 IM session key" });
  }, [copy]);

  const setFilter = useCallback(<Key extends keyof PairingFilters>(
    key: Key,
    value: PairingFilters[Key],
  ) => {
    setFilters((current) => ({ ...current, [key]: value }));
  }, []);
  const clearFeedback = useCallback(() => setFeedback(null), []);
  const closeCreate = useCallback(() => setCreateOpen(false), []);
  const openCreate = useCallback(() => setCreateOpen(true), []);
  const reportCreateError = useCallback((message: string) => {
    setFeedback({ tone: "error", title: "新增失败", message });
  }, []);

  return {
    activeCount: countPairingStatus(items, "active"),
    agents,
    busy: pendingAction !== null,
    clearFeedback,
    closeCreate,
    confirmDelete,
    copySessionKey,
    createOpen,
    deleteTarget,
    feedback,
    filters,
    groups,
    items,
    loading,
    openCreate,
    pairingCreated,
    pendingCount: countPairingStatus(items, "pending"),
    refresh,
    reportCreateError,
    requestDelete: setDeleteTarget,
    setFilter,
    updatePairing,
    visibleItems,
  };
}
