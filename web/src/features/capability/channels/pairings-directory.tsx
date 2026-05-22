"use client";

import { Check, Filter, RefreshCw, ShieldCheck, Trash2, X } from "lucide-react";
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
import { UiBadge } from "@/shared/ui/badge";
import type { UiBadgeTone } from "@/shared/ui/badge-styles";
import { UiButton, UiIconButton } from "@/shared/ui/button";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
import { UiField } from "@/shared/ui/form-control";
import { UiPanel } from "@/shared/ui/panel";
import { UiSelectMenu } from "@/shared/ui/select-menu";
import { UiStateBlock } from "@/shared/ui/state-block";
import {
  WorkspaceSurfaceHeader,
  WorkspaceSurfaceToolbarAction,
} from "@/shared/ui/workspace/surface/workspace-surface-header";
import { WorkspaceSurfaceScaffold } from "@/shared/ui/workspace/surface/workspace-surface-scaffold";
import type { Agent } from "@/types/agent/agent";

const CHANNEL_LABELS: Record<ImChannelType, string> = {
  dingtalk: "钉钉",
  wechat: "微信",
  feishu: "飞书",
  telegram: "Telegram",
  discord: "Discord",
};

const STATUS_LABELS: Record<ImPairingStatus, string> = {
  pending: "待处理",
  active: "已授权",
  disabled: "已停用",
  rejected: "已拒绝",
};

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

export function PairingsDirectory() {
  const [items, set_items] = useState<PairingView[]>([]);
  const [agents, set_agents] = useState<Agent[]>([]);
  const [status, set_status] = useState<ImPairingStatus | "">("");
  const [channel, set_channel] = useState<ImChannelType | "">("");
  const [loading, set_loading] = useState(true);
  const [busy_id, set_busy_id] = useState<string | null>(null);
  const [feedback, set_feedback] = useState<{ tone: "success" | "error"; title: string; message: string } | null>(null);

  const filtered_count = items.length;
  const pending_count = useMemo(() => items.filter((item) => item.status === "pending").length, [items]);

  const refresh = useCallback(async () => {
    set_loading(true);
    try {
      const [next_items, next_agents] = await Promise.all([
        list_pairings_api({ channel_type: channel, status }),
        get_agents(),
      ]);
      set_items(next_items);
      set_agents(next_agents);
    } catch (error) {
      set_feedback({ tone: "error", title: "加载失败", message: error instanceof Error ? error.message : "配对列表加载失败" });
    } finally {
      set_loading(false);
    }
  }, [channel, status]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  const update_pairing = async (item: PairingView, next: { status?: ImPairingStatus; agent_id?: string }) => {
    set_busy_id(item.pairing_id);
    try {
      const updated = await update_pairing_api(item.pairing_id, next);
      set_items((current) => current.map((value) => value.pairing_id === updated.pairing_id ? updated : value));
      set_feedback({ tone: "success", title: "配对已更新", message: `${updated.external_name || updated.external_ref} 已保存` });
    } catch (error) {
      set_feedback({ tone: "error", title: "更新失败", message: error instanceof Error ? error.message : "配对更新失败" });
    } finally {
      set_busy_id(null);
    }
  };

  const delete_pairing = async (item: PairingView) => {
    if (!window.confirm(`确认删除 ${item.external_name || item.external_ref} 的配对吗？`)) {
      return;
    }
    set_busy_id(item.pairing_id);
    try {
      await delete_pairing_api(item.pairing_id);
      set_items((current) => current.filter((value) => value.pairing_id !== item.pairing_id));
      set_feedback({ tone: "success", title: "配对已删除", message: `${item.external_name || item.external_ref} 已移除` });
    } catch (error) {
      set_feedback({ tone: "error", title: "删除失败", message: error instanceof Error ? error.message : "配对删除失败" });
    } finally {
      set_busy_id(null);
    }
  };

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
            density="compact"
            leading={<ShieldCheck className="h-4 w-4" />}
            subtitle="审批 IM 用户与群聊访问智能体的关系。未授权对象只会进入待处理，不会直接触发智能体。"
            title="配对授权"
            trailing={(
              <WorkspaceSurfaceToolbarAction onClick={() => void refresh()}>
                <RefreshCw className="h-3.5 w-3.5" />
                刷新
              </WorkspaceSurfaceToolbarAction>
            )}
          />
        )}
        stable_gutter
      >
        <div className="mx-auto w-full max-w-[1180px] px-6 py-5">
          <UiPanel class_name="mb-5 flex flex-wrap items-center gap-3" padding="sm">
            <Filter className="h-4 w-4 text-(--icon-default)" />
            <UiSelectMenu
              aria_label="筛选消息渠道"
              class_name="w-[148px]"
              on_change={(value) => set_channel(value as ImChannelType | "")}
              options={[
                { value: "", label: "全部渠道" },
                ...Object.entries(CHANNEL_LABELS).map(([key, label]) => ({
                  value: key,
                  label,
                })),
              ]}
              size="sm"
              value={channel}
            />
            <UiSelectMenu
              aria_label="筛选配对状态"
              class_name="w-[148px]"
              on_change={(value) => set_status(value as ImPairingStatus | "")}
              options={[
                { value: "", label: "全部状态" },
                ...Object.entries(STATUS_LABELS).map(([key, label]) => ({
                  value: key,
                  label,
                })),
              ]}
              size="sm"
              value={status}
            />
            <div className="ml-auto text-[12px] font-semibold text-(--text-muted)">
              {filtered_count} 个配对 · {pending_count} 个待处理
            </div>
          </UiPanel>

          {loading ? (
            <UiStateBlock description="正在同步外部 IM 用户与群聊的授权状态。" size="sm" title="加载配对授权..." />
          ) : items.length === 0 ? (
            <UiStateBlock
              description="外部 IM 用户或群首次发消息后，会在这里等待授权。"
              icon={<ShieldCheck className="h-6 w-6 text-(--icon-default)" />}
              size="md"
              title="暂无配对请求"
            />
          ) : (
            <div className="space-y-3">
              {items.map((item) => (
                <UiPanel
                  class_name="grid grid-cols-[minmax(0,1.4fr)_minmax(180px,0.8fr)_minmax(160px,0.7fr)_auto] items-center gap-4 max-xl:grid-cols-1"
                  key={item.pairing_id}
                >
                  <div className="min-w-0">
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <UiBadge>{CHANNEL_LABELS[item.channel_type]}</UiBadge>
                      <UiBadge tone={status_tone(item.status)}>
                        {STATUS_LABELS[item.status]}
                      </UiBadge>
                      <UiBadge>{item.chat_type === "group" ? "群聊" : "用户"}</UiBadge>
                    </div>
                    <div className="mt-2 truncate text-[17px] font-bold text-(--text-strong)">
                      {item.external_name || format_target(item)}
                    </div>
                    <div className="mt-1 truncate text-[12px] text-(--text-muted)">{format_target(item)}</div>
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

                  <div className="text-[12px] leading-5 text-(--text-muted)">
                    <div>来源：{item.source === "ingress" ? "首次消息" : item.source}</div>
                    <div>更新：{new Date(item.updated_at).toLocaleString()}</div>
                  </div>

                  <div className="flex items-center justify-end gap-2">
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
                      onClick={() => void delete_pairing(item)}
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
          )}
        </div>
      </WorkspaceSurfaceScaffold>

      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
