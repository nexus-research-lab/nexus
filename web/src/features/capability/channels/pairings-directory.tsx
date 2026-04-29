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
import { cn } from "@/lib/utils";
import { FeedbackBannerStack, type FeedbackBannerItem } from "@/shared/ui/feedback/feedback-banner-stack";
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

function status_class(status: ImPairingStatus) {
  switch (status) {
  case "active":
    return "bg-[#ecfdf3] text-[#067647]";
  case "pending":
    return "bg-[#fff7e6] text-[#b54708]";
  case "rejected":
    return "bg-[#fef3f2] text-[#b42318]";
  default:
    return "bg-[#f6f7f9] text-[#667085]";
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
        body_class_name="px-6 py-5"
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
        <div className="mb-5 flex flex-wrap items-center gap-3 rounded-[16px] border border-[#e7e8ec] bg-white px-4 py-3">
          <Filter className="h-4 w-4 text-[#8d95a3]" />
          <select
            className="h-9 rounded-[10px] border border-[#e1e4e9] bg-white px-3 text-[13px] font-semibold text-[#30333c] outline-none focus:border-[#9bdab8]"
            onChange={(event) => set_channel(event.target.value as ImChannelType | "")}
            value={channel}
          >
            <option value="">全部渠道</option>
            {Object.entries(CHANNEL_LABELS).map(([key, label]) => (
              <option key={key} value={key}>{label}</option>
            ))}
          </select>
          <select
            className="h-9 rounded-[10px] border border-[#e1e4e9] bg-white px-3 text-[13px] font-semibold text-[#30333c] outline-none focus:border-[#9bdab8]"
            onChange={(event) => set_status(event.target.value as ImPairingStatus | "")}
            value={status}
          >
            <option value="">全部状态</option>
            {Object.entries(STATUS_LABELS).map(([key, label]) => (
              <option key={key} value={key}>{label}</option>
            ))}
          </select>
          <div className="ml-auto text-[12px] font-semibold text-[#8d95a3]">
            {filtered_count} 个配对 · {pending_count} 个待处理
          </div>
        </div>

        {loading ? (
          <div className="flex h-40 items-center justify-center text-[13px] text-[#8d95a3]">加载配对授权...</div>
        ) : items.length === 0 ? (
          <div className="flex h-56 flex-col items-center justify-center rounded-[18px] border border-dashed border-[#d8dbe3] bg-white text-center">
            <ShieldCheck className="mb-3 h-8 w-8 text-[#a7adba]" />
            <div className="text-[16px] font-black text-[#30333c]">暂无配对请求</div>
            <div className="mt-1 text-[13px] text-[#8d95a3]">外部 IM 用户或群首次发消息后，会在这里等待授权。</div>
          </div>
        ) : (
          <div className="space-y-3">
            {items.map((item) => (
              <article
                className="grid grid-cols-[minmax(0,1.4fr)_minmax(180px,0.8fr)_minmax(160px,0.7fr)_auto] items-center gap-4 rounded-[16px] border border-[#e7e8ec] bg-white px-5 py-4 shadow-[0_1px_0_rgba(15,23,42,0.02)] max-xl:grid-cols-1"
                key={item.pairing_id}
              >
                <div className="min-w-0">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="rounded-full bg-[#f2f4f7] px-2.5 py-1 text-[12px] font-black text-[#4b5565]">
                      {CHANNEL_LABELS[item.channel_type]}
                    </span>
                    <span className={cn("rounded-full px-2.5 py-1 text-[12px] font-black", status_class(item.status))}>
                      {STATUS_LABELS[item.status]}
                    </span>
                    <span className="rounded-full bg-[#f8fafc] px-2.5 py-1 text-[12px] font-semibold text-[#667085]">
                      {item.chat_type === "group" ? "群聊" : "用户"}
                    </span>
                  </div>
                  <div className="mt-2 truncate text-[17px] font-black text-[#20222a]">
                    {item.external_name || format_target(item)}
                  </div>
                  <div className="mt-1 truncate text-[12px] text-[#8d95a3]">{format_target(item)}</div>
                </div>

                <label className="block text-[12px] font-semibold text-[#777d8a]">
                  处理智能体
                  <select
                    className="mt-2 h-10 w-full rounded-[12px] border border-[#e1e4e9] bg-white px-3 text-[13px] font-semibold text-[#30333c] outline-none focus:border-[#9bdab8]"
                    disabled={busy_id === item.pairing_id}
                    onChange={(event) => void update_pairing(item, { agent_id: event.target.value })}
                    value={item.agent_id}
                  >
                    {agents.map((agent) => (
                      <option key={agent.agent_id} value={agent.agent_id}>{agent.name}</option>
                    ))}
                  </select>
                </label>

                <div className="text-[12px] text-[#8d95a3]">
                  <div>来源：{item.source === "ingress" ? "首次消息" : item.source}</div>
                  <div className="mt-1">更新：{new Date(item.updated_at).toLocaleString()}</div>
                </div>

                <div className="flex items-center justify-end gap-2">
                  {item.status !== "active" ? (
                    <button
                      className="flex h-9 items-center gap-1.5 rounded-[10px] bg-[#12b76a] px-3 text-[12px] font-black text-white transition hover:bg-[#0e9f5d] disabled:opacity-60"
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void update_pairing(item, { status: "active" })}
                      type="button"
                    >
                      <Check className="h-3.5 w-3.5" />
                      通过
                    </button>
                  ) : null}
                  {item.status === "pending" ? (
                    <button
                      className="flex h-9 items-center gap-1.5 rounded-[10px] bg-[#f04438] px-3 text-[12px] font-black text-white transition hover:bg-[#d92d20] disabled:opacity-60"
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void update_pairing(item, { status: "rejected" })}
                      type="button"
                    >
                      <X className="h-3.5 w-3.5" />
                      拒绝
                    </button>
                  ) : null}
                  {item.status === "active" ? (
                    <button
                      className="h-9 rounded-[10px] border border-[#e1e4e9] px-3 text-[12px] font-black text-[#4b5565] transition hover:bg-[#f7f8fa] disabled:opacity-60"
                      disabled={busy_id === item.pairing_id}
                      onClick={() => void update_pairing(item, { status: "disabled" })}
                      type="button"
                    >
                      停用
                    </button>
                  ) : null}
                  <button
                    className="flex h-9 w-9 items-center justify-center rounded-[10px] border border-[#e1e4e9] text-[#98a0ad] transition hover:bg-[#fef3f2] hover:text-[#b42318] disabled:opacity-60"
                    disabled={busy_id === item.pairing_id}
                    onClick={() => void delete_pairing(item)}
                    title="删除"
                    type="button"
                  >
                    <Trash2 className="h-4 w-4" />
                  </button>
                </div>
              </article>
            ))}
          </div>
        )}
      </WorkspaceSurfaceScaffold>

      <FeedbackBannerStack items={feedback_items} />
    </>
  );
}
