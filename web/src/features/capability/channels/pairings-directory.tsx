"use client";

import { Check, Copy, Filter, Plus, RefreshCw, ShieldCheck, Trash2, Users, X } from "lucide-react";
import { useCallback, useEffect, useMemo, useState } from "react";

import { get_agents } from "@/lib/api/agent-manage-api";
import {
  delete_pairing_api,
  ImChannelType,
  ImPairingStatus,
  list_pairings_api,
  PairingView,
  update_pairing_api,
} from "@/lib/api/channel-api";
import { useCopyToClipboard } from "@/hooks/ui/use-copy-to-clipboard";
import { useI18n } from "@/shared/i18n/i18n-context";
import { UiBadge } from "@/shared/ui/badge";
import type { UiBadgeTone } from "@/shared/ui/badge-styles";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { ConfirmDialog } from "@/shared/ui/dialog/confirm-dialog";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiField } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/select-menu";
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

import { notify_capability_summary_mutated } from "../capability-summary-events";

import { CreatePairingDialog } from "./pairing-create-dialog";
import { CHANNEL_LABELS, CHANNEL_OPTIONS, CHAT_TYPE_OPTIONS, STATUS_LABELS } from "./pairing-options";

function status_tone(status: ImPairingStatus): UiBadgeTone {
  switch (status) {
  case "active":
    return "success";
  case "pending":
    return "warning";
  case "rejected":
    return "danger";
  default:
    return "default";
  }
}

function format_target(item: PairingView) {
  const thread = item.thread_id ? ` / ${item.thread_id}` : "";
  return `${item.external_ref}${thread}`;
}

function chat_type_label(item: PairingView) {
  return CHAT_TYPE_OPTIONS.find((option) => option.value === item.chat_type)?.label ?? item.chat_type;
}

function binding_key(item: PairingView) {
  return [
    CHANNEL_LABELS[item.channel_type] ?? item.channel_type,
    item.account_id || "default",
    chat_type_label(item),
    item.external_ref,
    item.thread_id || "-",
  ].join(" / ");
}

function session_key_for_pairing(item: PairingView) {
  return item.session_key || "";
}

export function PairingsDirectory() {
  const { t } = useI18n();
  const { copy } = useCopyToClipboard();
  const [items, set_items] = useState<PairingView[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [status, set_status] = useState<ImPairingStatus | "">("");
  const [channel, set_channel] = useState<ImChannelType | "">("");
  const [agent_id, set_agent_id] = useState("");
  const [query, set_query] = useState("");
  const [loading, set_loading] = useState(true);
  const [busy_id, set_busy_id] = useState<string | null>(null);
  const [create_open, set_create_open] = useState(false);
  const [delete_target, set_delete_target] = useState<PairingView | null>(null);
  const [feedback, set_feedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const visible_items = useMemo(() => {
    const normalized_query = query.trim().toLowerCase();
    if (!normalized_query) {
      return items;
    }
    return items.filter((item) =>
      (item.external_name ?? "").toLowerCase().includes(normalized_query)
      || item.external_ref.toLowerCase().includes(normalized_query)
      || (item.account_id ?? "").toLowerCase().includes(normalized_query)
      || (item.thread_id ?? "").toLowerCase().includes(normalized_query)
      || session_key_for_pairing(item).toLowerCase().includes(normalized_query)
      || (item.agent_name ?? "").toLowerCase().includes(normalized_query)
      || (CHANNEL_LABELS[item.channel_type] ?? item.channel_type).toLowerCase().includes(normalized_query),
    );
  }, [items, query]);
  const filtered_count = visible_items.length;
  const pending_count = useMemo(() => items.filter((item) => item.status === "pending").length, [items]);
  const active_count = useMemo(() => items.filter((item) => item.status === "active").length, [items]);
  const grouped_items = useMemo(() => {
    const agent_names = new Map(agents.map((agent) => [agent.agent_id, agent.name]));
    const groups = new Map<string, { agent_id: string; agent_name: string; items: PairingView[] }>();
    visible_items.forEach((item) => {
      const key = item.agent_id;
      const existing = groups.get(key);
      if (existing) {
        existing.items.push(item);
        return;
      }
      groups.set(key, {
        agent_id: key,
        agent_name: item.agent_name || agent_names.get(key) || key,
        items: [item],
      });
    });
    return Array.from(groups.values()).sort((left, right) => left.agent_name.localeCompare(right.agent_name));
  }, [agents, visible_items]);

  const refresh = useCallback(async () => {
    set_loading(true);
    try {
      const [next_items, next_agents] = await Promise.all([
        list_pairings_api({ channel_type: channel, status, agent_id }),
        get_agents(),
      ]);
      set_items(next_items);
      set_agents(next_agents);
    } catch (error) {
      set_feedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "配对列表加载失败" });
    } finally {
      set_loading(false);
    }
  }, [agent_id, channel, status]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const update_pairing = async (item: PairingView, next: { status?: ImPairingStatus; agent_id?: string }) => {
    set_busy_id(item.pairing_id);
    try {
      const updated = await update_pairing_api(item.pairing_id, next);
      set_items((current) => current.map((value) => value.pairing_id === updated.pairing_id ? updated : value));
      notify_capability_summary_mutated({ source: "pairings", action: "update", pairing_id: updated.pairing_id });
      set_feedback({ tone: "success", title: "配对已更新", message: `${updated.external_name || updated.external_ref} 已保存` });
    } catch (error) {
      set_feedback({ tone: "error", title: "更新失败", message: error instanceof Error ? error.message : "配对更新失败" });
    } finally {
      set_busy_id(null);
    }
  };

  const delete_pairing = async (item: PairingView) => {
    set_busy_id(item.pairing_id);
    try {
      await delete_pairing_api(item.pairing_id);
      set_items((current) => current.filter((value) => value.pairing_id !== item.pairing_id));
      notify_capability_summary_mutated({ source: "pairings", action: "delete", pairing_id: item.pairing_id });
      set_feedback({ tone: "success", title: "配对已删除", message: `${item.external_name || item.external_ref} 已移除` });
    } catch (error) {
      set_feedback({ tone: "error", title: "删除失败", message: error instanceof Error ? error.message : "配对删除失败" });
    } finally {
      set_busy_id(null);
    }
  };

  const confirm_delete_pairing = () => {
    if (!delete_target) {
      return;
    }
    const target = delete_target;
    set_delete_target(null);
    void delete_pairing(target);
  };

  const handle_pairing_created = useCallback((item: PairingView) => {
    set_feedback({ tone: "success", title: "配对已新增", message: `${item.external_name || item.external_ref} 已创建` });
    void refresh();
  }, [refresh]);

  const copy_session_key = useCallback(async (item: PairingView) => {
    const key = session_key_for_pairing(item);
    const ok = await copy(key);
    set_feedback(ok
      ? { tone: "success", title: "Session 已复制", message: key }
      : { tone: "error", title: "复制失败", message: "无法复制 IM session key" });
  }, [copy]);

  const feedback_items: FeedbackBannerItem[] = feedback
    ? [{
        key: "pairings-feedback",
        tone: feedback.tone,
        title: feedback.title,
        message: feedback.message,
        on_dismiss: () => set_feedback(null),
      }]
    : [];

  return (
    <>
      <WorkspaceSurfaceScaffold
        body_scrollable
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
                  onClick={() => set_create_open(true)}
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
        stable_gutter
      >
        <CapabilityPageLayout
          description={t("capability.pairings_intro_description")}
          title={t("capability.pairings_intro_title")}
        >
          <CapabilityFilterBar>
            <CapabilityFilterSearchInput
              on_change={set_query}
              placeholder={t("capability.pairings_search_placeholder")}
              value={query}
            />
            <CapabilityFilterSelect
              aria_label={t("capability.pairings_filter_channel_aria")}
              leading={<Filter className="h-3.5 w-3.5" />}
              on_change={(value) => set_channel(value as ImChannelType | "")}
              options={[
                { value: "", label: "全部渠道" },
                ...CHANNEL_OPTIONS,
              ]}
              value={channel}
            />
            <CapabilityFilterSelect
              aria_label={t("capability.pairings_filter_status_aria")}
              on_change={(value) => set_status(value as ImPairingStatus | "")}
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
              aria_label="按处理智能体筛选"
              class_name="sm:w-[220px]"
              leading={<Users className="h-3.5 w-3.5" />}
              on_change={set_agent_id}
              options={[
                { value: "", label: "全部智能体" },
                ...agents.map((agent) => ({
                  value: agent.agent_id,
                  label: agent.name,
                })),
              ]}
              value={agent_id}
            />
            <div className="shrink-0 text-[12px] font-semibold text-(--text-muted) sm:ml-auto">
              {filtered_count} 个配对 · {active_count} 个已授权 · {pending_count} 个待处理
            </div>
          </CapabilityFilterBar>

          {loading ? (
            <UiStateBlock description="正在同步外部 IM 用户与群聊的授权状态。" size="sm" title="加载配对..." />
          ) : visible_items.length === 0 ? (
            <UiStateBlock
              description="外部 IM 用户或群首次发消息后，会在这里等待授权。"
              icon={<ShieldCheck className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title="暂无配对请求"
            />
          ) : (
            <div className="space-y-5">
              {grouped_items.map((group) => (
                <section className="space-y-2.5" key={group.agent_id}>
                  <div className="flex items-center justify-between border-b border-(--divider-subtle-color) pb-2">
                    <div className="min-w-0">
                      <h2 className="truncate text-[15px] font-semibold text-(--text-strong)">
                        {group.agent_name}
                      </h2>
                      <p className="truncate text-[12px] text-(--text-muted)">
                        {group.items.length} 个外部对象绑定到此智能体
                      </p>
                    </div>
                    <UiBadge tone="default">{group.agent_id}</UiBadge>
                  </div>
                  <div className="space-y-2.5">
                    {group.items.map((item) => (
                      <UiPanel
                        class_name="grid grid-cols-[minmax(0,1.2fr)_minmax(210px,0.8fr)_minmax(260px,1fr)_auto] items-center gap-4 max-2xl:grid-cols-[minmax(0,1fr)_minmax(240px,1fr)] max-lg:grid-cols-1"
                        key={item.pairing_id}
                      >
                        <div className="min-w-0">
                          <div className="flex min-w-0 flex-wrap items-center gap-2">
                            <UiBadge>{CHANNEL_LABELS[item.channel_type] ?? item.channel_type}</UiBadge>
                            <UiBadge tone={status_tone(item.status)}>
                              {STATUS_LABELS[item.status]}
                            </UiBadge>
                            <UiBadge>{chat_type_label(item)}</UiBadge>
                            {item.account_id ? <UiBadge tone="default">{item.account_id}</UiBadge> : null}
                          </div>
                          <div className="mt-2 truncate text-[16px] font-bold text-(--text-strong)">
                            {item.external_name || format_target(item)}
                          </div>
                          <div className="mt-1 truncate font-mono text-[12px] text-(--text-muted)">
                            {format_target(item)}
                          </div>
                          {item.account_id ? (
                            <div className="mt-1 truncate font-mono text-[11px] text-(--text-soft)" title={item.account_id}>
                              account: {item.account_id}
                            </div>
                          ) : null}
                        </div>

                        <UiField class_name="min-w-0" label="处理智能体">
                          <UiSelectMenu
                            aria_label="选择配对处理智能体"
                            disabled={busy_id === item.pairing_id}
                            on_change={(value) => void update_pairing(item, { agent_id: value })}
                            options={agents.map((agent) => ({
                              value: agent.agent_id,
                              label: agent.name,
                            }))}
                            size="sm"
                            value={item.agent_id}
                          />
                        </UiField>

                        <div className="min-w-0 space-y-1.5 text-[12px] leading-5 text-(--text-muted)">
                          <div className="min-w-0">
                            <div className="text-[11px] font-semibold uppercase text-(--text-soft)">绑定键</div>
                            <div className="truncate font-mono text-(--text-default)" title={binding_key(item)}>
                              {binding_key(item)}
                            </div>
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-1.5 text-[11px] font-semibold uppercase text-(--text-soft)">
                              <span>IM Session</span>
                              <UiIconButton
                                class_name="h-6 w-6"
                                disabled={busy_id === item.pairing_id}
                                onClick={() => void copy_session_key(item)}
                                size="sm"
                                title="复制 IM session key"
                                type="button"
                                variant="ghost"
                              >
                                <Copy className="h-3.5 w-3.5" />
                              </UiIconButton>
                            </div>
                            <div className="truncate font-mono text-(--text-default)" title={session_key_for_pairing(item)}>
                              {session_key_for_pairing(item)}
                            </div>
                          </div>
                          <div className="truncate">
                            来源：{item.source === "ingress" ? "首次消息" : item.source} · 更新：{new Date(item.updated_at).toLocaleString()}
                          </div>
                        </div>

                        <div className="flex items-center justify-end gap-2 max-lg:justify-start">
                          {item.status !== "active" ? (
                            <UiButton
                              disabled={busy_id === item.pairing_id}
                              onClick={() => void update_pairing(item, { status: "active" })}
                              size="sm"
                              tone="primary"
                              type="button"
                              variant="solid"
                            >
                              <Check className="h-3.5 w-3.5" />
                              通过
                            </UiButton>
                          ) : null}
                          {item.status === "pending" ? (
                            <UiButton
                              disabled={busy_id === item.pairing_id}
                              onClick={() => void update_pairing(item, { status: "rejected" })}
                              size="sm"
                              tone="danger"
                              type="button"
                              variant="surface"
                            >
                              <X className="h-3.5 w-3.5" />
                              拒绝
                            </UiButton>
                          ) : null}
                          {item.status === "active" ? (
                            <UiButton
                              disabled={busy_id === item.pairing_id}
                              onClick={() => void update_pairing(item, { status: "disabled" })}
                              size="sm"
                              type="button"
                            >
                              停用
                            </UiButton>
                          ) : null}
                          <UiIconButton
                            disabled={busy_id === item.pairing_id}
                            onClick={() => set_delete_target(item)}
                            size="lg"
                            title="删除"
                            tone="danger"
                            type="button"
                            variant="ghost"
                          >
                            <Trash2 className="h-4 w-4" />
                          </UiIconButton>
                        </div>
                      </UiPanel>
                    ))}
                  </div>
                </section>
              ))}
            </div>
          )}
        </CapabilityPageLayout>
      </WorkspaceSurfaceScaffold>

      {create_open ? (
        <CreatePairingDialog
          agents={agents}
          on_close={() => set_create_open(false)}
          on_created={handle_pairing_created}
          on_error={(message) => set_feedback({ tone: "error", title: "新增失败", message })}
        />
      ) : null}

      <FeedbackBannerStack items={feedback_items} />
      <ConfirmDialog
        confirm_text="删除配对"
        is_open={delete_target !== null}
        message={delete_target
          ? `确认删除 ${delete_target.external_name || delete_target.external_ref} 的配对吗？删除后该外部对象需要重新授权。`
          : ""}
        on_cancel={() => set_delete_target(null)}
        on_confirm={confirm_delete_pairing}
        title="删除配对"
        variant="danger"
      />
    </>
  );
}
