"use client";

import { Filter, Plus, RefreshCw, ShieldCheck, Users } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { getAgents } from "@/lib/api/agent-manage-api";
import {
  deletePairingApi,
  ImChannelType,
  ImPairingStatus,
  listPairingsApi,
  PairingView,
  updatePairingApi,
} from "@/lib/api/channel-api";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useI18n } from "@/shared/i18n/i18n-context";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  CapabilityFilterBar,
  CapabilityFilterSearchInput,
  CapabilityFilterSelect,
  CapabilityPageLayout,
} from "@/features/capability/shared/capability-page-layout";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";

import { notifyCapabilitySummaryMutated } from "../capability-summary-events";

import { CreatePairingDialog } from "./pairing-create-dialog";
import {
  PairingList,
  type PairingGroup,
} from "./pairing-list";
import { CHANNEL_LABELS, CHANNEL_OPTIONS, STATUS_LABELS } from "./pairing-options";

function sessionKeyForPairing(item: PairingView) {
  return item.session_key || "";
}

export function PairingsDirectory() {
  const { t } = useI18n();
  const { copy } = useCopyToClipboard();
  const [items, setItems] = useState<PairingView[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [status, setStatus] = useState<ImPairingStatus | "">("");
  const [channel, setChannel] = useState<ImChannelType | "">("");
  const [agentId, setAgentId] = useState("");
  const [query, setQuery] = useState("");
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);
  const [createOpen, setCreateOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<PairingView | null>(null);
  const [feedback, setFeedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const visibleItems = useMemo(() => {
    const normalizedQuery = query.trim().toLowerCase();
    if (!normalizedQuery) {
      return items;
    }
    return items.filter((item) =>
      (item.external_name ?? "").toLowerCase().includes(normalizedQuery)
      || item.external_ref.toLowerCase().includes(normalizedQuery)
      || (item.account_id ?? "").toLowerCase().includes(normalizedQuery)
      || (item.thread_id ?? "").toLowerCase().includes(normalizedQuery)
      || sessionKeyForPairing(item).toLowerCase().includes(normalizedQuery)
      || (item.agent_name ?? "").toLowerCase().includes(normalizedQuery)
      || (CHANNEL_LABELS[item.channel_type] ?? item.channel_type).toLowerCase().includes(normalizedQuery),
    );
  }, [items, query]);
  const filteredCount = visibleItems.length;
  const pendingCount = useMemo(() => items.filter((item) => item.status === "pending").length, [items]);
  const activeCount = useMemo(() => items.filter((item) => item.status === "active").length, [items]);
  const groupedItems = useMemo(() => {
    const agentNames = new Map(agents.map((agent) => [agent.agent_id, agent.name]));
    const groups = new Map<string, PairingGroup>();
    visibleItems.forEach((item) => {
      const key = item.agent_id;
      const existing = groups.get(key);
      if (existing) {
        existing.items.push(item);
        return;
      }
      groups.set(key, {
        agent_id: key,
        agent_name: item.agent_name || agentNames.get(key) || key,
        items: [item],
      });
    });
    return Array.from(groups.values()).sort((left, right) => left.agent_name.localeCompare(right.agent_name));
  }, [agents, visibleItems]);

  const refresh = useCallback(async () => {
    setLoading(true);
    try {
      const [nextItems, nextAgents] = await Promise.all([
        listPairingsApi({ channel_type: channel, status, agent_id: agentId }),
        getAgents(),
      ]);
      setItems(nextItems);
      setAgents(nextAgents);
    } catch (error) {
      setFeedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "配对列表加载失败" });
    } finally {
      setLoading(false);
    }
  }, [agentId, channel, status]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const updatePairing = async (item: PairingView, next: { status?: ImPairingStatus; agent_id?: string }) => {
    setBusyId(item.pairing_id);
    try {
      const updated = await updatePairingApi(item.pairing_id, next);
      setItems((current) => current.map((value) => value.pairing_id === updated.pairing_id ? updated : value));
      notifyCapabilitySummaryMutated({ source: "pairings", action: "update", pairing_id: updated.pairing_id });
      setFeedback({ tone: "success", title: "配对已更新", message: `${updated.external_name || updated.external_ref} 已保存` });
    } catch (error) {
      setFeedback({ tone: "error", title: "更新失败", message: error instanceof Error ? error.message : "配对更新失败" });
    } finally {
      setBusyId(null);
    }
  };

  const deletePairing = async (item: PairingView) => {
    setBusyId(item.pairing_id);
    try {
      await deletePairingApi(item.pairing_id);
      setItems((current) => current.filter((value) => value.pairing_id !== item.pairing_id));
      notifyCapabilitySummaryMutated({ source: "pairings", action: "delete", pairing_id: item.pairing_id });
      setFeedback({ tone: "success", title: "配对已删除", message: `${item.external_name || item.external_ref} 已移除` });
    } catch (error) {
      setFeedback({ tone: "error", title: "删除失败", message: error instanceof Error ? error.message : "配对删除失败" });
    } finally {
      setBusyId(null);
    }
  };

  const confirmDeletePairing = () => {
    if (!deleteTarget) {
      return;
    }
    const target = deleteTarget;
    setDeleteTarget(null);
    void deletePairing(target);
  };

  const handlePairingCreated = useCallback((item: PairingView) => {
    setFeedback({ tone: "success", title: "配对已新增", message: `${item.external_name || item.external_ref} 已创建` });
    void refresh();
  }, [refresh]);

  const copySessionKey = useCallback(async (item: PairingView) => {
    const key = sessionKeyForPairing(item);
    const ok = await copy(key);
    setFeedback(ok
      ? { tone: "success", title: "Session 已复制", message: key }
      : { tone: "error", title: "复制失败", message: "无法复制 IM session key" });
  }, [copy]);

  const feedbackItems: FeedbackBannerItem[] = feedback
    ? [{
        key: "pairings-feedback",
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
            badge={t("capability.pairings_badge", { count: items.length })}
            density="compact"
            leading={<ShieldCheck className="h-4 w-4" />}
            subtitle={t("capability.pairings_subtitle")}
            title={t("capability.pairings")}
            trailing={(
              <>
                <WorkspaceSurfaceToolbarAction
                  disabled={agents.length === 0}
                  onClick={() => setCreateOpen(true)}
                  title={agents.length === 0 ? "需要先创建智能体" : "新增 IM 配对"}
                  tone="primary"
                >
                  <Plus className="h-3.5 w-3.5" />
                  新增配对
                </WorkspaceSurfaceToolbarAction>
                <WorkspaceSurfaceToolbarAction onClick={() => void refresh()}>
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
              onChange={setQuery}
              placeholder={t("capability.pairings_search_placeholder")}
              value={query}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.pairings_filter_channel_aria")}
              leading={<Filter className="h-3.5 w-3.5" />}
              onChange={(value) => setChannel(value as ImChannelType | "")}
              options={[
                { value: "", label: "全部渠道" },
                ...CHANNEL_OPTIONS,
              ]}
              value={channel}
            />
            <CapabilityFilterSelect
              ariaLabel={t("capability.pairings_filter_status_aria")}
              onChange={(value) => setStatus(value as ImPairingStatus | "")}
              options={[
                { value: "", label: "全部状态" },
                ...Object.entries(STATUS_LABELS).map(([key, label]) => ({
                  value: key,
                  label,
                })),
              ]}
              value={status}
            />
            <CapabilityFilterSelect
              ariaLabel="按处理智能体筛选"
              className="sm:w-[220px]"
              leading={<Users className="h-3.5 w-3.5" />}
              onChange={setAgentId}
              options={[
                { value: "", label: "全部智能体" },
                ...agents.map((agent) => ({
                  value: agent.agent_id,
                  label: agent.name,
                })),
              ]}
              value={agentId}
            />
            <div className="shrink-0 text-[12px] font-semibold text-(--text-muted) sm:ml-auto">
              {filteredCount} 个配对 · {activeCount} 个已授权 · {pendingCount} 个待处理
            </div>
          </CapabilityFilterBar>

          {loading ? (
            <UiStateBlock description="正在同步外部 IM 用户与群聊的授权状态。" size="sm" title="加载配对..." />
          ) : visibleItems.length === 0 ? (
            <UiStateBlock
              description="外部 IM 用户或群首次发消息后，会在这里等待授权。"
              icon={<ShieldCheck className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title="暂无配对请求"
            />
          ) : (
            <PairingList
              agents={agents}
              busyId={busyId}
              groups={groupedItems}
              onCopySessionKey={copySessionKey}
              onDeletePairing={setDeleteTarget}
              onUpdatePairing={updatePairing}
            />
          )}
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      {createOpen ? (
        <CreatePairingDialog
          agents={agents}
          onClose={() => setCreateOpen(false)}
          onCreated={handlePairingCreated}
          onError={(message) => setFeedback({ tone: "error", title: "新增失败", message })}
        />
      ) : null}

      <FeedbackBannerStack items={feedbackItems} />
      <ConfirmDialog
        confirmText="删除配对"
        isOpen={deleteTarget !== null}
        message={deleteTarget
          ? `确认删除 ${deleteTarget.external_name || deleteTarget.external_ref} 的配对吗？删除后该外部对象需要重新授权。`
          : ""}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={confirmDeletePairing}
        title="删除配对"
        variant="danger"
      />
    </>
  );
}
