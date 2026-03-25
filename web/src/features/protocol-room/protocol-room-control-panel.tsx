"use client";

import { ArrowRight, Pause, Play, Send, TimerReset } from "lucide-react";

import { ProtocolRunControlOperation, ProtocolRunDetail } from "@/types";

import {
  renderDefinitionDescription,
  renderDefinitionLabel,
  renderPhaseLabel,
  renderStatusLabel,
  renderWinnerLabel,
} from "./protocol-room-helpers";

interface ProtocolRoomControlPanelProps {
  detail: ProtocolRunDetail;
  is_loading: boolean;
  is_busy_control: boolean;
  pending_current_phase_requests_count: number;
  pending_requests_for_viewer_count: number;
  viewer_agent_id: string | null;
  force_phase_name: string;
  remaining_phases: string[];
  inject_channel_id: string;
  inject_message: string;
  on_change_force_phase: (value: string) => void;
  on_control: (operation: ProtocolRunControlOperation, payload?: Record<string, any>) => Promise<void>;
  on_send_room_message: () => Promise<void>;
}

export function ProtocolRoomControlPanel({
  detail,
  is_loading,
  is_busy_control,
  pending_current_phase_requests_count,
  pending_requests_for_viewer_count,
  viewer_agent_id,
  force_phase_name,
  remaining_phases,
  inject_channel_id,
  inject_message,
  on_change_force_phase,
  on_control,
  on_send_room_message,
}: ProtocolRoomControlPanelProps) {
  return (
    <aside className="neo-card flex min-h-0 flex-col rounded-[28px] p-4">
      <div className="border-b border-white/55 pb-4">
        <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
          房间状态
        </p>
        <div className="mt-3 grid gap-3">
          <div className="rounded-[22px] border border-white/60 bg-white/40 px-4 py-3">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-700/48">
              当前阶段
            </p>
            <p className="mt-2 text-lg font-black tracking-[-0.04em] text-slate-950/90">
              {renderPhaseLabel(detail.run.current_phase)}
            </p>
            <p className="mt-1 text-xs text-slate-700/54">
              {renderStatusLabel(detail.run.status)}
            </p>
          </div>

          <div className="rounded-[22px] border border-white/60 bg-white/40 px-4 py-3">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-700/48">
              待你介入
            </p>
            <p className="mt-2 text-lg font-black tracking-[-0.04em] text-slate-950/90">
              {viewer_agent_id ? pending_requests_for_viewer_count : pending_current_phase_requests_count}
            </p>
            <p className="mt-1 text-xs text-slate-700/54">
              {viewer_agent_id ? "与你当前视角相关的待处理动作" : "当前阶段的待处理动作"}
            </p>
          </div>

          <div className="rounded-[22px] border border-white/60 bg-white/40 px-4 py-3">
            <p className="text-xs font-semibold uppercase tracking-[0.14em] text-slate-700/48">
              天数 / 胜者
            </p>
            <p className="mt-2 text-lg font-black tracking-[-0.04em] text-slate-950/90">
              第 {detail.run.state?.day ?? 1} 天
            </p>
            <p className="mt-1 text-xs text-slate-700/54">
              {detail.run.state?.winner ? `胜者：${renderWinnerLabel(detail.run.state.winner)}` : "当前还没有胜者"}
            </p>
          </div>
        </div>

        <div className="mt-4 flex flex-wrap gap-2">
          <button
            className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white transition hover:translate-y-[-1px]"
            disabled={is_busy_control || is_loading}
            onClick={() => void on_control(detail.run.status === "paused" ? "resume" : "pause")}
            type="button"
          >
            {detail.run.status === "paused" ? <Play className="h-4 w-4" /> : <Pause className="h-4 w-4" />}
            {detail.run.status === "paused" ? "恢复" : "暂停"}
          </button>
          <button
            className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900 transition hover:translate-y-[-1px]"
            disabled={is_busy_control || !force_phase_name}
            onClick={() => void on_control("force_transition", { phase_name: force_phase_name })}
            type="button"
          >
            <ArrowRight className="h-4 w-4" />
            强制推进
          </button>
          <button
            className="rounded-full border border-rose-400/30 bg-rose-500/10 px-4 py-2 text-sm font-semibold text-rose-900/84 transition hover:translate-y-[-1px]"
            disabled={is_busy_control}
            onClick={() => void on_control("terminate_run")}
            type="button"
          >
            终止
          </button>
        </div>

        <div className="mt-3">
          <select
            className="neo-inset w-full rounded-[18px] px-3 py-2.5 text-sm text-slate-900/86 outline-none"
            onChange={(event) => on_change_force_phase(event.target.value)}
            value={force_phase_name}
          >
            <option value="">选择要强制进入的阶段</option>
            {remaining_phases.map((phase_name) => (
              <option key={phase_name} value={phase_name}>
                {renderPhaseLabel(phase_name)}
              </option>
            ))}
          </select>
        </div>
      </div>

      <div className="mt-4 min-h-0 flex-1 overflow-y-auto pr-1 scrollbar-hide">
        <div className="space-y-4">
          <section className="rounded-[24px] border border-white/60 bg-white/40 px-4 py-4">
            <div className="flex items-center justify-between gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
                当前剧本
              </p>
              <div className="rounded-full bg-white/74 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/56">
                {renderDefinitionLabel(detail.definition.slug)}
              </div>
            </div>
            <p className="mt-3 text-sm leading-6 text-slate-700/64">
              {renderDefinitionDescription(detail.definition.slug, detail.definition.description)}
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              {detail.definition.phases.map((phase_name) => (
                <span
                  key={phase_name}
                  className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] ${
                    phase_name === detail.run.current_phase
                      ? "bg-slate-950 text-white"
                      : "bg-white/74 text-slate-700/60"
                  }`}
                >
                  {renderPhaseLabel(phase_name)}
                </span>
              ))}
            </div>
          </section>

          <section className="rounded-[24px] border border-white/60 bg-white/40 px-4 py-4">
            <div className="flex items-center justify-between gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
                最近结算
              </p>
              <TimerReset className="h-4 w-4 text-slate-600/60" />
            </div>
            <div className="mt-3 space-y-3 text-sm leading-6 text-slate-700/64">
              <div>
                <p className="font-semibold text-slate-950/84">夜晚</p>
                <p>
                  {detail.run.state?.last_night_result?.deaths?.length
                    ? `淘汰成员：${detail.run.state.last_night_result.deaths.join(", ")}`
                    : "当前还没有可见的夜晚淘汰结果。"}
                </p>
              </div>
              <div>
                <p className="font-semibold text-slate-950/84">投票</p>
                <p>
                  {detail.run.state?.last_vote_result?.target
                    ? `投票淘汰：${detail.run.state.last_vote_result.target}`
                    : "当前还没有完成的公开投票。"}
                </p>
              </div>
            </div>
          </section>

          <section className="rounded-[24px] border border-white/60 bg-white/40 px-4 py-4">
            <div className="flex items-center justify-between gap-3">
              <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-700/48">
                快速广播
              </p>
              <div className="rounded-full bg-white/74 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-700/56">
                系统辅助
              </div>
            </div>
            <p className="mt-3 text-sm leading-6 text-slate-700/64">
              如果你希望以主持人身份补充一条推进消息，可以直接发送当前房间输入框里的内容。
            </p>
            <button
              className="mt-4 inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white transition hover:translate-y-[-1px]"
              disabled={!inject_channel_id || !inject_message.trim() || is_busy_control}
              onClick={() => void on_send_room_message()}
              type="button"
            >
              <Send className="h-4 w-4" />
              发送当前消息
            </button>
          </section>
        </div>
      </div>
    </aside>
  );
}
