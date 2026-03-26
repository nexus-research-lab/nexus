"use client";

import { Bot, Sparkles } from "lucide-react";

import { Agent } from "@/types/agent";

interface RoomAgentAboutViewProps {
  agent: Agent;
}

export function RoomAgentAboutView({ agent }: RoomAgentAboutViewProps) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-hidden bg-transparent">
      <div className="border-b workspace-divider px-5 py-2.5 xl:px-6">
        <div className="mx-auto w-full max-w-[760px]">
          <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-slate-700/44">
            About
          </p>
          <h2 className="mt-1 text-[16px] font-black tracking-[-0.04em] text-slate-950/88">
            {agent.name}
          </h2>
        </div>
      </div>

      <div className="soft-scrollbar min-h-0 flex-1 overflow-y-auto px-4 py-3 sm:px-5 xl:px-6">
        <div className="mx-auto w-full max-w-[760px] rounded-[18px] border border-white/22 bg-white/10 px-5 py-5">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-full border border-white/28 bg-white/18 text-slate-900/78">
              <Bot className="h-5 w-5" />
            </div>
            <div>
              <p className="text-base font-semibold text-slate-950/88">{agent.name}</p>
              <p className="text-[13px] text-slate-700/56">单成员协作对象</p>
            </div>
          </div>

          <div className="mt-4 grid gap-2 sm:grid-cols-2">
            <div className="rounded-[14px] border border-white/18 bg-white/10 px-4 py-3.5">
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/46">
                Model
              </p>
              <p className="mt-1.5 text-[13px] font-semibold text-slate-950/84">
                {agent.options.model || "inherit"}
              </p>
            </div>
            <div className="rounded-[14px] border border-white/18 bg-white/10 px-4 py-3.5">
              <p className="text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-700/46">
                Skills
              </p>
              <p className="mt-1.5 inline-flex items-center gap-2 text-[13px] font-semibold text-slate-950/84">
                <Sparkles className="h-4 w-4 text-sky-600" />
                {agent.options.skills_enabled ? "已启用" : "未启用"}
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
