"use client";

import { AlertTriangle, Eye, Play, RadioTower, RefreshCcw, Sparkles, Users } from "lucide-react";

import { RoomAggregate, RoomMemberRecord, WebSocketState } from "@/types";

interface ProtocolRoomEmptyStateProps {
  room: RoomAggregate;
  room_agent_members: RoomMemberRecord[];
  ws_state: WebSocketState;
  error: string | null;
  on_create_run: () => Promise<unknown>;
  on_refresh: () => Promise<unknown>;
}

export function ProtocolRoomEmptyState({
  room,
  room_agent_members,
  ws_state,
  error,
  on_create_run,
  on_refresh,
}: ProtocolRoomEmptyStateProps) {
  return (
    <div className="relative flex min-h-0 flex-1 flex-col px-4 py-4 sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute inset-x-[12%] top-[8%] h-36 rounded-full bg-[radial-gradient(circle,rgba(120,170,255,0.2),transparent_72%)] blur-3xl" />
      <section className="panel-surface relative flex min-h-0 flex-1 flex-col overflow-hidden rounded-[34px] px-4 py-4 sm:px-6 sm:py-6">
        <header className="relative z-10 flex flex-col gap-4 border-b border-white/55 pb-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2 text-[11px] uppercase tracking-[0.18em] text-slate-700/52">
              <span className="neo-pill rounded-full px-3 py-1">多人协作房间</span>
              <span>{room.room.room_type === "room" ? "多人协作" : "单聊"}</span>
              <span>{ws_state === "connected" ? "实时同步" : "轮询回退"}</span>
            </div>
            <h1 className="mt-3 text-[30px] font-black tracking-[-0.05em] text-slate-950/92">
              {room.room.name || room.room.id}
            </h1>
            <p className="mt-2 max-w-3xl text-sm leading-6 text-slate-700/62">
              这是一个真正的多人协作房间。你可以和多个 agent 处在同一个房间里，共享频道、共享流程、共享阶段推进。
            </p>
          </div>

          <div className="flex flex-wrap items-center gap-2">
            <button
              className="neo-pill inline-flex items-center gap-2 rounded-full px-4 py-2 text-sm font-semibold text-slate-900 transition hover:translate-y-[-1px]"
              onClick={() => void on_refresh()}
              type="button"
            >
              <RefreshCcw className="h-4 w-4" />
              刷新
            </button>
            <button
              className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-4 py-2 text-sm font-semibold text-white shadow-[0_18px_38px_rgba(15,23,42,0.22)] transition hover:translate-y-[-1px]"
              onClick={() => void on_create_run()}
              type="button"
            >
              <Sparkles className="h-4 w-4" />
              启动演示协作
            </button>
          </div>
        </header>

        {error ? (
          <div className="neo-card-flat relative z-10 mt-4 flex items-start gap-3 rounded-[24px] border border-rose-400/25 px-4 py-3 text-sm text-rose-900/84">
            <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-rose-500" />
            <span>{error}</span>
          </div>
        ) : null}

        <div className="relative z-10 grid flex-1 gap-4 py-6 xl:grid-cols-[320px_minmax(0,1fr)_320px]">
          <div className="neo-card flex min-h-0 flex-col rounded-[28px] p-5">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/50">
              <Users className="h-4 w-4" />
              房间成员
            </div>
            <div className="mt-4 space-y-3">
              {room_agent_members.map((member) => (
                <div
                  key={member.id}
                  className="rounded-[22px] border border-white/60 bg-white/44 px-4 py-3"
                >
                  <p className="text-sm font-semibold text-slate-950/90">{member.member_agent_id}</p>
                  <p className="mt-1 text-xs text-slate-700/56">等待进入协议协作</p>
                </div>
              ))}
            </div>
          </div>

          <div className="neo-card flex min-h-0 flex-col rounded-[28px] p-6">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/50">
              <RadioTower className="h-4 w-4" />
              房间流
            </div>
            <h2 className="mt-5 text-[28px] font-black tracking-[-0.05em] text-slate-950/92">
              这个房间还没有开始协作流程
            </h2>
            <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-700/62">
              启动后你会看到成员频道、真实房间流、动作卡和当前阶段状态。狼人杀只是一个演示场景，最终体验会更像你和多个 agent 在同一个房间里一起互动协作。
            </p>
            <div className="mt-8 flex flex-wrap gap-3">
              <button
                className="inline-flex items-center gap-2 rounded-full bg-slate-950 px-5 py-2.5 text-sm font-semibold text-white transition hover:translate-y-[-1px]"
                onClick={() => void on_create_run()}
                type="button"
              >
                <Play className="h-4 w-4" />
                启动演示协作
              </button>
              <div className="neo-card-flat flex items-center gap-2 rounded-full px-4 py-2 text-xs font-semibold uppercase tracking-[0.15em] text-slate-700/56">
                <Eye className="h-3.5 w-3.5" />
                真实房间，多方协作
              </div>
            </div>
          </div>

          <div className="neo-card flex min-h-0 flex-col rounded-[28px] p-5">
            <div className="flex items-center gap-2 text-xs font-semibold uppercase tracking-[0.16em] text-slate-700/50">
              <RadioTower className="h-4 w-4" />
              协作说明
            </div>
            <div className="mt-4 space-y-4 text-sm leading-6 text-slate-700/64">
              <p>左边会展示成员与频道，让你先知道房间里都有谁。</p>
              <p>中间会展示真实房间流，所有系统播报、阶段推进、成员发言和私密事件都会在对应频道出现。</p>
              <p>右边只放辅助控制与状态，不再把“控制台”当主角。</p>
            </div>
          </div>
        </div>
      </section>
    </div>
  );
}
