"use client";

import { useEffect, useMemo, useState } from "react";
import { Users, X } from "lucide-react";

import { Agent } from "@/types/agent";

interface CreateRoomDialogProps {
  agents: Agent[];
  default_title: string;
  error?: string | null;
  is_open: boolean;
  is_submitting?: boolean;
  on_cancel: () => void;
  on_confirm: (payload: {
    title: string;
    agent_ids: string[];
    mode: "open";
    goal?: string;
    ruleset_slug?: string;
  }) => Promise<void> | void;
}

export function CreateRoomDialog({
  agents,
  default_title,
  error = null,
  is_open,
  is_submitting = false,
  on_cancel,
  on_confirm,
}: CreateRoomDialogProps) {
  const [title, set_title] = useState(default_title);
  const [selected_agent_ids, set_selected_agent_ids] = useState<string[]>([]);
  const [goal, set_goal] = useState("");

  useEffect(() => {
    if (!is_open) {
      return;
    }
    set_title(default_title);
    set_selected_agent_ids((prev) => {
      if (prev.length) {
        return prev.filter((agent_id) => agents.some((agent) => agent.agent_id === agent_id));
      }
      return agents[0]?.agent_id ? [agents[0].agent_id] : [];
    });
    set_goal("");
  }, [agents, default_title, is_open]);

  const can_submit = useMemo(
    () => title.trim().length > 0 && selected_agent_ids.length > 0 && !is_submitting,
    [is_submitting, selected_agent_ids.length, title],
  );

  if (!is_open) {
    return null;
  }

  return (
    <div
      aria-labelledby="create-room-dialog-title"
      aria-modal="true"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 backdrop-blur-sm animate-in fade-in duration-150"
      role="dialog"
    >
      <div className="panel-surface w-full max-w-xl rounded-[30px] p-5 shadow-[0_32px_96px_rgba(15,23,42,0.16)] animate-in zoom-in-95 duration-150">
        <div className="flex items-start justify-between gap-3 border-b border-white/55 pb-3">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/46">
              Meeting Room
            </p>
            <h3 id="create-room-dialog-title" className="mt-2 text-xl font-black tracking-[-0.04em] text-slate-950/92">
              创建会议室式协作房间
            </h3>
          </div>
          <button
            aria-label="关闭"
            className="neo-pill rounded-full p-2 text-slate-700/56 transition hover:text-slate-950/90"
            onClick={on_cancel}
            type="button"
          >
            <X className="h-4 w-4" />
          </button>
        </div>

        <div className="mt-4 grid gap-4">
          {error ? (
            <div className="rounded-[18px] border border-rose-400/28 bg-rose-500/10 px-4 py-3 text-sm text-rose-900/84">
              {error}
            </div>
          ) : null}

          <div>
            <label className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              房间标题
            </label>
            <input
              className="neo-inset mt-2 w-full rounded-[18px] px-4 py-3 text-sm text-slate-950/86 outline-none"
              onChange={(event) => set_title(event.target.value)}
              placeholder="输入 room 标题"
              value={title}
            />
          </div>

          <div className="rounded-[20px] border border-white/60 bg-white/38 px-4 py-4">
            <p className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              房间模式
            </p>
            <div className="mt-2 rounded-[18px] border border-slate-950/16 bg-slate-950/7 px-4 py-4">
              <p className="text-sm font-semibold text-slate-950/90">开放协作会议室</p>
              <p className="mt-2 text-xs leading-5 text-slate-700/60">
                当前公开入口只保留真实可用的会议室协作模式，支持广播、@成员、任务分配和 workspace 回流。
              </p>
            </div>
          </div>

          <div>
            <label className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
              协作目标
            </label>
            <textarea
              className="neo-inset mt-2 min-h-[108px] w-full rounded-[18px] px-4 py-3 text-sm leading-6 text-slate-950/86 outline-none"
              onChange={(event) => set_goal(event.target.value)}
              placeholder="例如：帮我拉 3 个智能体一起重构 room runtime，并把关键实现推进到可运行。"
              value={goal}
            />
          </div>

          <div>
            <div className="flex items-center gap-2">
              <Users className="h-4 w-4 text-slate-700/54" />
              <label className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-700/48">
                成员
              </label>
            </div>
            <div className="mt-3 grid max-h-[320px] gap-3 overflow-y-auto pr-1 scrollbar-hide">
              {agents.map((agent) => {
                const is_selected = selected_agent_ids.includes(agent.agent_id);
                return (
                  <button
                    key={agent.agent_id}
                    className={`rounded-[22px] border px-4 py-3 text-left transition ${
                      is_selected
                        ? "border-slate-950/18 bg-slate-950/8"
                        : "border-white/60 bg-white/42 hover:bg-white/56"
                    }`}
                    onClick={() => {
                      set_selected_agent_ids((prev) => (
                        prev.includes(agent.agent_id)
                          ? prev.filter((item) => item !== agent.agent_id)
                          : [...prev, agent.agent_id]
                      ));
                    }}
                    type="button"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <div>
                        <p className="text-sm font-semibold text-slate-950/90">{agent.name}</p>
                        <p className="mt-1 text-xs text-slate-700/56">{agent.agent_id}</p>
                      </div>
                      <div
                        className={`rounded-full px-3 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${
                          is_selected
                            ? "bg-slate-950 text-white"
                            : "bg-white/70 text-slate-700/58"
                        }`}
                      >
                        {is_selected ? "已选择" : "点击加入"}
                      </div>
                    </div>
                  </button>
                );
              })}

              {!agents.length ? (
                <div className="rounded-[22px] border border-dashed border-white/60 bg-white/24 px-4 py-5 text-sm leading-6 text-slate-700/58">
                  当前还没有可加入 room 的普通成员，先创建一个 agent。
                </div>
              ) : null}
            </div>
          </div>
        </div>

        <div className="mt-5 flex items-center justify-between gap-3 border-t border-white/55 pt-4">
          <p className="text-xs text-slate-700/56">
            已选择 {selected_agent_ids.length} 个成员
          </p>
          <div className="flex gap-2">
            <button
              className="neo-pill rounded-full px-4 py-2 text-sm font-semibold text-slate-900"
              onClick={on_cancel}
              type="button"
            >
              取消
            </button>
            <button
              className="rounded-full bg-slate-950 px-5 py-2 text-sm font-semibold text-white transition disabled:cursor-not-allowed disabled:opacity-50"
              disabled={!can_submit}
              onClick={() => void on_confirm({
                title: title.trim(),
                agent_ids: selected_agent_ids,
                mode: "open",
                goal: goal.trim(),
              })}
              type="button"
            >
              {is_submitting ? "创建中..." : "创建 room"}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
